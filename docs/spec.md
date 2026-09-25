# Spec: pdf-main

**Estado:** Aprobado

## Objective

`pdf-main` es el **orquestador de la lógica de negocio** del sistema *Parse Documents Fast*. Es el único servicio con ruta pública detrás de `pdf-infra` (Traefik + Redis) y el único que habla con `pdf-persistence`.

Recibe la subida de un documento (PDF o Markdown), lo valida, y según el formato: encola la extracción (PDF) o persiste directo (Markdown). Expone el resultado persistido para consulta y descarga, donde el **formato canónico es Markdown** (ADR-0005). No extrae, no convierte, no persiste y no valida por sí mismo: **coordina** a `pdf-validator`, `pdf-extractor`, `pdf-converter` y `pdf-persistence`.

**Usuarios:** el CLI (legacy, en desuso a futuro) y una futura web que consumirá esta API vía HTTP/HTTPS.

**Éxito:** una subida devuelve `200`; el PDF queda `pending` y su Markdown llega asíncronamente, el Markdown se persiste directo; la descarga devuelve Markdown tal cual o PDF convertido; los errores viajan en RFC 9457.

---

## Alcance (qué hace y qué NO hace)

### Hace

1. Exponer la API HTTP pública del sistema (subida, listado, detalle, borrado, descarga).
2. Orquestar la subida de PDF: validar → detectar duplicados → crear registro `pending` → encolar en `queue:extraction`.
3. Orquestar la subida de Markdown: validar → detectar duplicados → persistir directo (síncrono, sin cola — ADR-0005).
4. Orquestar la descarga: pedir Markdown → devolverlo tal cual, o pasarlo por `pdf-converter` (Markdown→PDF).
5. Consumir `queue:extraction-results` y actualizar la persistencia.
6. Traducir errores y timeouts de downstream a RFC 9457 coherente para el cliente.

### No hace

- **No** extrae estructura de PDFs ni arma Markdown (eso es `pdf-extractor`, que absorbió a `pdf-transformator` — ADR-0005).
- **No** convierte Markdown a PDF (eso es `pdf-converter`).
- **No** valida contenido (eso es `pdf-validator`).
- **No** persiste ni habla con MongoDB directamente (eso es `pdf-persistence`).
- **No** toca disco en ningún punto del flujo — todo en RAM (restricción del profesor).

---

## Tech Stack

| Componente | Elección | Justificación |
|---|---|---|
| Lenguaje | Go (1.24+) | Concurrencia real para atender subidas/descargas simultáneas y el consumer de cola |
| Router HTTP | `github.com/go-chi/chi/v5` | Router ligero, ergonomía de middlewares y path params |
| Cliente Redis | `go-redis/v9` | Streams con consumer groups (`XREADGROUP`/`XACK`/`XADD`) |
| HTTP client | `net/http` stdlib | Para hablar con validator/persistence/converter |
| Circuit breaker | `github.com/sony/gobreaker` | Envuelve las llamadas HTTP internas a downstream (resiliencia interna) |
| MongoDB | *ninguno* | `pdf-main` no toca Mongo: pasa por `pdf-persistence` |

**Dependencias externas:** `github.com/go-chi/chi/v5`, `github.com/redis/go-redis/v9` y `github.com/sony/gobreaker`. Módulo: `github.com/Parse-Documents-Fast/pdf-main`.

---

## Commands

```bash
# Build
go build ./...

# Test
go test ./...

# Lint / vet
go vet ./...
gofmt -l .          # no debe listar ningún archivo

# Dev (run local)
go run ./cmd/pdf-main
```

---

## Project Structure

```
cmd/pdf-main/main.go        → entrypoint: carga config, arma adaptadores, levanta server + consumer
internal/
  config/config.go          → config desde variables de entorno
  problem/problem.go        → helpers RFC 9457 (ProblemDetails, WriteProblem)
  dto/                      → DTOs del wire (snake_case) entre pdf-main y los demás servicios
    documents.go            → contrato público + contrato de pdf-persistence
    validator.go
    extractor.go
    converter.go
  orchestrator/             → NÚCLEO de negocio (puro, sin HTTP ni cola)
    upload.go
    download.go
    query.go
    delete.go
  httpapi/                  → ADAPTADORES HTTP (handlers, mux, middlewares)
    router.go
    handler_upload.go
    handler_download.go
    handler_query.go
  clients/                  → ADAPTADORES: clientes HTTP a servicios downstream
    validator.go
    persistence.go
    converter.go
  queue/                    → ADAPTADORES de cola (producer + consumer de resultados)
    producer.go
    consumer.go
docs/
  spec.md                   → este documento
  plan-pdf-main.md
  standards/                → ADRs referenciados
tasks/                      → plan y tareas (fases PLAN/TASKS)
```

La separación núcleo/transporte es por **ADR-0004**: `orchestrator` son funciones puras que dependen de interfaces (`ports`); `httpapi`, `clients` y `queue` son adaptadores delgados que las invocan/implementan.

---

## Code Style

Go idiomático, `gofmt`, tabs. JSON tags siempre `snake_case` (ADR-0002). El núcleo depende de interfaces, no de implementaciones:

```go
// internal/orchestrator — núcleo puro, sin HTTP ni cola.
type Ports struct {
    Validator   clients.Validator
    Persistence clients.Persistence
    Queue       queue.Producer
}

// Núcleo: función pura, devuelve valores y errores de dominio.
func Submit(ctx context.Context, p Ports, content []byte, filename string) (dto.PdfSummary, error) {
    v, err := p.Validator.Validate(ctx, content, filename)
    if err != nil {
        return dto.PdfSummary{}, err // → 400
    }
    dup, err := p.Persistence.FindByChecksum(ctx, v.Checksum)
    if err != nil {
        return dto.PdfSummary{}, err
    }
    if dup != nil {
        return dto.PdfSummary{}, fmt.Errorf("%w: %s", ErrDuplicate, dup.ID)
    }

    switch v.OriginalFormat {
    case dto.FormatPDF:
        // PDF: async — crear pending y encolar extracción (ADR-0004/0005)
        rec, err := p.Persistence.Create(ctx, dto.PersistCreateRequest{
            Title:          titleOrFilename(filename),
            OriginalFormat: v.OriginalFormat,
            Checksum:       v.Checksum,
            Status:         dto.StatusPending,
        })
        if err != nil {
            return dto.PdfSummary{}, err
        }
        if err := p.Queue.PublishExtraction(ctx, dto.ExtractionJob{
            PdfID: rec.ID, Filename: filename, ContentBase64: base64(content),
        }); err != nil {
            return dto.PdfSummary{}, err
        }
        return rec.Summary(), nil // → 200 (pending)

    case dto.FormatMarkdown:
        // Markdown: sync — persistir directo con contenido (ADR-0005)
        rec, err := p.Persistence.Create(ctx, dto.PersistCreateRequest{
            Title:          titleOrFilename(filename),
            OriginalFormat: v.OriginalFormat,
            Checksum:       v.Checksum,
            Status:         dto.StatusDone,
            Content:        string(content),
        })
        if err != nil {
            return dto.PdfSummary{}, err
        }
        return rec.Summary(), nil // → 200 (done)
    }
    return dto.PdfSummary{}, ErrInvalid
}
```

Convenciones:
- Errores de dominio como sentinelas (`ErrInvalid`, `ErrDuplicate`, `ErrNotFound`, `ErrDownstream`) que los adaptadores mapean a status HTTP.
- Los adaptadores traducen a RFC 9457 (ADR-0001); el núcleo no conoce HTTP.
- Binario siempre base64 en el wire (ADR-0002); texto plano (Markdown) viaja como string.
- Contextos con timeout en toda llamada downstream.

---

## Testing Strategy

- Framework: `testing` stdlib + `net/http/httptest` para handlers.
- Los clientes (`clients/*`) y el producer/consumer de cola se mockean con interfaces; el núcleo se testea contra mocks.
- Tests de handlers validan: status code, body RFC 9457, headers (`Content-Disposition`), y mapeo de errores de dominio → HTTP.
- Niveles:
  - **Unit** (núcleo + mapeo de errores): tabla-driven.
  - **HTTP** (handlers con `httptest` + mocks): subida 200, duplicado 409, not-found 404, descarga con formato.
  - **Integración manual** (opcional, no en CI): contra `pdf-infra` + los demás servicios levantados.
- Cobertura objetivo: núcleo y mapeo de errores ≥ 90%; no se persigue cobertura sobre adaptadores de Redis real.

---

## DTOs (contrato de wire — snake_case, ADR-0002)

`pdf-main` es la fuente de estos contratos. Los demás servicios los referencian desde acá.

### Contrato público (cliente ↔ pdf-main)

```jsonc
// PdfSummary — respuesta de POST /api/pdfs (200, PDF o Markdown) y GET /api/pdfs (200)
{
  "id": "665f1a2b3c4d5e6f7a8b9c0d",
  "title": "informe",
  "original_format": "pdf",            // "pdf" | "markdown"
  "checksum": "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3",
  "status": "pending",                  // "pending" | "done" | "failed"
  "created_at": "2026-09-18T10:00:00Z"
}

// PdfDocument — respuesta de GET /api/pdfs/{id} (200): PdfSummary + content
{
  "id": "...",
  "title": "...",
  "original_format": "pdf",
  "checksum": "...",
  "status": "done",
  "created_at": "...",
  "content": "# Título\n\ntexto…",      // Markdown; null mientras pending / failed
  "error": null                          // string corto, solo cuando status="failed"
}
```

### `pdf-validator` (HTTP síncrono)

```jsonc
// request
{ "filename": "informe.pdf", "content_base64": "JVBERi0xLjQK…" }

// response (200)
{ "original_format": "pdf", "checksum": "a94a8fe5…" }
// error → RFC 9457 (400 archivo inválido)
```

### `pdf-extractor` (cola)

```jsonc
// job en queue:extraction
{ "pdf_id": "665f…", "filename": "informe.pdf", "content_base64": "JVBERi0xLjQK…" }

// resultado en queue:extraction-results (Markdown ya armado, ADR-0005)
{ "pdf_id": "665f…", "status": "done", "content": "# Título\n\ntexto…" }
// o, ante error:
{ "pdf_id": "665f…", "status": "failed", "error": { /* RFC 9457 */ } }
```

### `pdf-converter` (HTTP síncrono — solo descarga)

```jsonc
// request de descarga (Markdown → PDF)
{ "content": "# Título\n\ntexto…" }

// response de descarga (200)
{ "content_base64": "JVBERi0xLjQK…", "mime_type": "application/pdf" }
```

> `pdf-converter` ya no tiene ingesta (ADR-0005): `queue:conversion` y `queue:conversion-results` **no existen**.

### `pdf-persistence` (HTTP síncrono)

```jsonc
// create (POST)
{ "title": "informe", "original_format": "pdf", "checksum": "…", "status": "pending" }

// record (respuesta de create / get / find)
{
  "id": "665f…",
  "title": "informe",
  "original_format": "pdf",
  "checksum": "…",
  "status": "pending",
  "content": null,          // Markdown (string), no binario
  "error": null,
  "created_at": "2026-09-18T10:00:00Z"
}

// update (PATCH) — cuando llega un resultado de cola
{ "content": "# Título\n\ntexto…", "status": "done" }
// o, ante fallo:
{ "status": "failed", "error": "No se pudo extraer texto del PDF" }
```

> **Nota de coordinación:** el plan de `pdf-persistence` lista el modelo como `content, checksum, original_format, title, created_at`. Este spec le **agrega `status` y `error`** (necesarios porque la subida de PDF es asíncrona y la web debe poder mostrar el motivo del fallo). Quedan como requisito para `pdf-persistence`. La **limpieza/TTL de documentos `failed` es responsabilidad de `pdf-persistence`, no de `pdf-main`**.

---

## Endpoints públicos de pdf-main

| Método | Path | Éxito | Errores (RFC 9457) |
|---|---|---|---|
| `POST` | `/api/pdfs` | `200` `PdfSummary` (PDF: `pending`; Markdown: `done`) | `400` inválido, `409` duplicado (con `existing_id`), `503` downstream |
| `GET` | `/api/pdfs` | `200` `[PdfSummary]` | — |
| `GET` | `/api/pdfs/{id}` | `200` `PdfDocument` | `404` no existe |
| `DELETE` | `/api/pdfs/{id}` | `204` | `404` no existe |
| `GET` | `/api/pdfs/{id}/download?format=pdf\|markdown` | `200` archivo (`Content-Disposition: attachment; filename="{title}.{ext}"`) | `404`, `409` aún `pending`, `422` si `failed`, `400` formato inválido |

- Subida: `multipart/form-data` con `file` (binario) + `title` (opcional, default = nombre del archivo sin extensión).
- `format` en la descarga es opcional; default `markdown` (el formato canónico, sin conversión).
- CORS: `allow all` (la futura web lo necesita); se puede ajustar después.

---

## Flujo de subida (dos caminos — ADR-0005)

```
POST /api/pdfs
  → pdf-validator.Validate (HTTP sync)           → clasifica + checksum  → 400 si inválido
  → pdf-persistence.FindByChecksum (HTTP sync)   → 409 si duplicado
  ── según original_format:
  │ pdf:
  │   → pdf-persistence.Create(status=pending)
  │   → queue:extraction (job con content_base64)
  │   → 200 PdfSummary(status=pending)
  │
  │ markdown:
  │   → pdf-persistence.Create(status=done, content=markdown)   // síncrono, sin cola
  │   → 200 PdfSummary(status=done)
```

```
Después (async, solo PDF):
  pdf-main consume queue:extraction-results
    → pdf-persistence.Update(content=markdown, status="done" | "failed")
```

## Flujo de descarga (sync)

```
GET /api/pdfs/{id}/download?format=pdf|markdown
  → pdf-persistence.Get(id)            → content (Markdown) + title   → 404 si no existe
  → status != "done"                   → 409 (pending) / 422 (failed)
  → format=markdown: devolver content tal cual (text/markdown)        // sin converter
  → format=pdf:     pdf-converter.Convert (Markdown → PDF)            // HTTP sync
  → devolver archivo (Content-Disposition)
```

---

## Resiliencia interna (circuit breaker en Go)

Además del middleware de circuit breaker de Traefik (borde público, en `pdf-infra`), `pdf-main` protege sus llamadas internas a downstream con `gobreaker`, uno por cliente HTTP (`validator`, `converter`, `persistence`):

- **Cerrado**: las llamadas fluyen normal; se contabilizan los fallos (5xx, timeout, error de red).
- **Abierto**: tras superar el umbral de fallos, `pdf-main` responde `503` RFC 9457 de inmediato, sin esperar el timeout ni tocar downstream.
- **Half-open**: tras `Timeout`, deja pasar un único request de prueba (`MaxRequests`); si falla vuelve a abrir, si funciona cierra.

Config por servicio (defaults de `gobreaker`):

| Parámetro | Default | Descripción |
|---|---|---|
| `MaxRequests` | `1` | Requests permitidos en estado half-open |
| `Timeout` | `30s` | Tiempo en abierto antes de pasar a half-open |
| `ReadyToTrip` | `ConsecutiveFailures > 5` | Condición que abre el breaker |

El breaker envuelve solo las llamadas internas de `clients/*`; el borde público (cliente → pdf-main) lo cubre Traefik.

## Configuración (variables de entorno)

| Variable | Default | Descripción |
|---|---|---|
| `HTTP_ADDR` | `:8000` | Puerto interno de escucha (detrás de Traefik, no expuesto al host) |
| `VALIDATOR_URL` | `http://pdf-validator:8000` | Base URL de `pdf-validator` |
| `PERSISTENCE_URL` | `http://pdf-persistence:8000` | Base URL de `pdf-persistence` |
| `CONVERTER_URL` | `http://pdf-converter:8000` | Base URL de `pdf-converter` |
| `REDIS_QUEUE_ADDR` | `redis-queue:6379` | Redis de colas (ADR-0004) |
| `MAX_FILE_SIZE_MB` | `10` | Tope de tamaño heredado del monolito |

**Streams (constantes, ADR-0004 + ADR-0005):** solo `queue:extraction` y `queue:extraction-results`, con consumer group y `XACK`. (`queue:conversion*` eliminados.)

**Traefik:** `pdf-main` declara en su `docker-compose.yml` los labels de enrutado (`Host(api.pdfmanager.local)`, entrypoint `websecure`, TLS) y encadena los middlewares que define `pdf-infra` por file provider:

```yaml
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.pdf-main.rule=Host(`api.pdfmanager.local`)"
  - "traefik.http.routers.pdf-main.entrypoints=websecure"
  - "traefik.http.routers.pdf-main.tls=true"
  - "traefik.http.routers.pdf-main.middlewares=rate-limit-redis@file,cb-documents@file"
```

El middleware `cb-documents` es el circuit breaker de Traefik (ya configurado en `pdf-infra`); `rate-limit-redis` es el rate limiter existente. `pdf-main` además declara `networks: [fast_pdf_network]` para conectarse al resto de servicios.

---

## Boundaries

- **Always:**
  - `gofmt` + `go test ./...` + `go vet ./...` antes de commit.
  - JSON tags `snake_case` (ADR-0002); binario en base64; texto (Markdown) como string.
  - Errores al cliente en RFC 9457 (ADR-0001).
  - Núcleo (`orchestrator`) sin conocimiento de HTTP ni cola (ADR-0004).
  - Todo en RAM: nunca escribir a disco.
  - Contextos con timeout en toda llamada downstream.

- **Ask first:**
  - Cambiar un field name de un DTO (es contrato compartido con otros servicios).
  - Agregar una dependencia de módulo nueva.
  - Cambiar nombres de streams / consumer groups.
  - Cambiar el modelo de persistencia (implica coordinar con `pdf-persistence`).
  - Bump de versión de Go.

- **Never:**
  - Commitear secretos / `.env`.
  - Escribir archivos temporales en disco.
  - Hablar con MongoDB directamente (siempre vía `pdf-persistence`).
  - Hablar con `pdf-extractor` directamente (solo vía cola).
  - Exponer puerto al host (solo accesible vía Traefik).

---

## Success Criteria

1. `POST /api/pdfs` con un PDF válido responde `200` (`status=pending`) y, tras el async, `GET /api/pdfs/{id}` muestra `status=done` con `content` (Markdown) poblado.
2. `POST /api/pdfs` con un Markdown válido responde `200` (`status=done`) con el `content` persistido síncronamente, sin pasar por cola.
3. Un archivo con magic bytes inválidos responde `400` RFC 9457 (`title: "Archivo inválido"`).
4. Subir el mismo contenido dos veces responde `409` con `existing_id`.
5. Un Markdown no publica nada en `queue:extraction` (persiste directo).
6. `GET /api/pdfs/{id}/download?format=pdf` devuelve `application/pdf` (vía `pdf-converter`); `format=markdown` devuelve `text/markdown` (passthrough); ambos con `Content-Disposition`.
7. Descargar un documento aún `pending` → `409`; uno `failed` → `422`.
8. `GET /api/pdfs/{id}` inexistente → `404`; `DELETE` → `204`.
9. Un timeout/5xx de un servicio downstream se traduce a `503` RFC 9457 (no un panic ni un stack trace).
10. `gofmt -l .` no reporta nada; `go test ./...` y `go vet ./...` pasan.
11. Con el breaker en estado abierto, una llamada a un servicio downstream falla rápido con `503` RFC 9457, sin esperar el timeout.

---

## Coordinación con otros repos

1. **`pdf-persistence`** debe agregar `status` y `error` a su modelo (ver nota en la sección DTOs) y es responsable de la limpieza/TTL de documentos `failed`.
2. **`pdf-infra`** ya expone los middlewares `rate-limit-redis` y `cb-documents`; `pdf-main` solo los referencia por labels.
3. **`pdf-extractor`** (fusionado con `pdf-transformator`, ADR-0005) produce `content` (Markdown) en `queue:extraction-results`, no HTML.
