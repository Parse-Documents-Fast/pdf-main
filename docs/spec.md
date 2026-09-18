# Spec: pdf-main

**Estado:** En revisión (fase SPECIFY — gateada por aprobación humana)

## Objective

`pdf-main` es el **orquestador de la lógica de negocio** del sistema *Parse Documents Fast*. Es el único servicio con ruta pública detrás de `pdf-infra` (Traefik + Redis) y el único que habla con `pdf-persistance`.

Recibe la subida de un documento (PDF o Markdown), lo valida, decide a qué servicio downstream derivarlo, encola el trabajo pesado (extracción o conversión) y expone el resultado persistido para consulta y descarga. No extrae, no convierte, no persiste y no valida por sí mismo: **coordina** a `pdf-validator`, `pdf-extractor`, `pdf-converter` y `pdf-persistance`.

**Usuarios:** el CLI (legacy, en desuso a futuro) y una futura web que consumirá esta API vía HTTP/HTTPS.

**Éxito:** una subida devuelve `202` con el recurso en estado `pending`; el contenido procesado llega asíncronamente y queda persistido como HTML; la descarga lo devuelve convertido a PDF o Markdown; los errores viajan en RFC 9457.

---

## Alcance (qué hace y qué NO hace)

### Hace

1. Exponer la API HTTP pública del sistema (subida, listado, detalle, borrado, descarga).
2. Orquestar la subida: validar → detectar duplicados → crear registro `pending` → encolar.
3. Orquestar la descarga: pedir HTML → convertir → devolver archivo.
4. Consumir los streams de resultados (`queue:extraction-results`, `queue:conversion-results`) y actualizar la persistencia.
5. Traducir errores y timeouts de downstream a RFC 9457 coherente para el cliente.

### No hace

- **No** extrae estructura de PDFs (eso es `pdf-extractor` + `pdf-transformator`).
- **No** convierte formatos (eso es `pdf-converter`).
- **No** valida contenido (eso es `pdf-validator`).
- **No** persiste ni habla con MongoDB directamente (eso es `pdf-persistance`).
- **No** toca disco en ningún punto del flujo — todo en RAM (restricción del profesor).

---

## Tech Stack

| Componente | Elección | Justificación |
|---|---|---|
| Lenguaje | Go (1.24+) | Concurrencia real para atender subidas/descargas simultáneas y N consumers de cola |
| Router HTTP | `net/http` stdlib (ServeMux con patrones método+ruta, Go 1.22+) | Sin dependencias externas; KISS. Suficiente para rutas con path params |
| Cliente Redis | `go-redis/v9` | Streams con consumer groups (`XREADGROUP`/`XACK`/`XADD`) |
| HTTP client | `net/http` stdlib | Para hablar con validator/persistance/converter |
| Circuit breaker | `github.com/sony/gobreaker` | Envuelve las llamadas HTTP internas a downstream (resiliencia interna) |
| MongoDB | *ninguno* | `pdf-main` no toca Mongo: pasa por `pdf-persistance` |

**Dependencias externas:** `github.com/redis/go-redis/v9` y `github.com/sony/gobreaker`. Módulo: `github.com/Parse-Documents-Fast/pdf-main`.

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
cmd/pdf-main/main.go        → entrypoint: carga config, arma adaptadores, levanta server + consumers
internal/
  config/config.go          → config desde variables de entorno
  problem/problem.go        → helpers RFC 9457 (ProblemDetails, WriteProblem)
  dto/                      → DTOs del wire (snake_case) entre pdf-main y los demás servicios
    documents.go            → contrato público + contrato de pdf-persistance
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
    persistance.go
    converter.go
  queue/                    → ADAPTADORES de cola (producer + consumers de resultados)
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
    rec, err := p.Persistence.Create(ctx, dto.PersistCreateRequest{
        Title:          titleOrFilename(filename),
        OriginalFormat: v.OriginalFormat,
        Checksum:       v.Checksum,
        Status:         dto.StatusPending,
    })
    if err != nil {
        return dto.PdfSummary{}, err
    }
    // encolar según el formato detectado
    switch v.OriginalFormat {
    case dto.FormatPDF:
        err = p.Queue.PublishExtraction(ctx, dto.ExtractionJob{...})
    case dto.FormatMarkdown:
        err = p.Queue.PublishConversion(ctx, dto.ConversionJob{...})
    }
    if err != nil {
        return dto.PdfSummary{}, err
    }
    return rec.Summary(), nil
}
```

Convenciones:
- Errores de dominio como sentinelas (`ErrDuplicate`, `ErrNotFound`, `ErrDownstream`) que los adaptadores mapean a status HTTP.
- Los adaptadores traducen a RFC 9457 (ADR-0001); el núcleo no conoce HTTP.
- Binario siempre base64 en el wire (ADR-0002); texto plano (HTML, Markdown) viaja como string.
- Contextos con timeout en toda llamada downstream.

---

## Testing Strategy

- Framework: `testing` stdlib + `net/http/httptest` para handlers.
- Los clientes (`clients/*`) y el producer/consumer de cola se mockean con interfaces; el núcleo se testea contra mocks.
- Tests de handlers validan: status code, body RFC 9457, headers (`Content-Disposition`), y mapeo de errores de dominio → HTTP.
- Niveles:
  - **Unit** (núcleo + mapeo de errores): tabla-driven.
  - **HTTP** (handlers con `httptest` + mocks): subida 202, duplicado 409, not-found 404, descarga con formato.
  - **Integración manual** (opcional, no en CI): contra `pdf-infra` + los demás servicios levantados.
- Cobertura objetivo: núcleo y mapeo de errores ≥ 90%; no se persigue cobertura sobre adaptadores de Redis real.

---

## DTOs (contrato de wire — snake_case, ADR-0002)

`pdf-main` es la fuente de estos contratos. Los demás servicios los referencian desde acá.

### Contrato público (cliente ↔ pdf-main)

```jsonc
// PdfSummary — respuesta de POST /api/pdfs (202) y GET /api/pdfs (200)
{
  "id": "665f1a2b3c4d5e6f7a8b9c0d",
  "title": "informe",
  "original_format": "pdf",            // "pdf" | "markdown"
  "checksum": "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3",
  "status": "pending",                  // "pending" | "done" | "failed"
  "created_at": "2026-09-18T10:00:00Z"
}

// PdfDocument — respuesta de GET /api/pdfs/{id} (200): PdfSummary + content_html
{
  "id": "...",
  "title": "...",
  "original_format": "pdf",
  "checksum": "...",
  "status": "done",
  "created_at": "...",
  "content_html": "<h1>Título</h1><p>texto…</p>"   // null mientras pending
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

// resultado en queue:extraction-results
{ "pdf_id": "665f…", "status": "done", "content_html": "<h1>…</h1>" }
// o, ante error:
{ "pdf_id": "665f…", "status": "failed", "error": { /* RFC 9457 */ } }
```

### `pdf-converter` (cola + HTTP síncrono)

```jsonc
// job en queue:conversion (ingesta, Markdown → HTML)
{ "pdf_id": "665f…", "filename": "nota.md", "content": "# Título\n…" }

// resultado en queue:conversion-results
{ "pdf_id": "665f…", "status": "done", "content_html": "<h1>Título</h1>…" }
// o { "pdf_id": "…", "status": "failed", "error": { /* RFC 9457 */ } }

// request de descarga (HTTP síncrono, HTML → formato)
{ "content_html": "<h1>…</h1>", "target_format": "pdf" }  // "pdf" | "markdown"

// response de descarga (200)
{ "content_base64": "JVBERi0xLjQK…", "mime_type": "application/pdf" }
```

### `pdf-persistance` (HTTP síncrono)

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
  "content_html": null,
  "created_at": "2026-09-18T10:00:00Z"
}

// update (PATCH) — cuando llega un resultado de cola
{ "content_html": "<h1>…</h1>", "status": "done" }
// o { "status": "failed" }
```

> **Nota de coordinación:** el plan de `pdf-persistance` lista el modelo como `content_html, checksum, original_format, title, created_at`. Este spec le **agrega `status`** (necesario porque el flujo es asíncrono). Queda como requisito para `pdf-persistance`.

---

## Endpoints públicos de pdf-main

| Método | Path | Éxito | Errores (RFC 9457) |
|---|---|---|---|
| `POST` | `/api/pdfs` | `202` `PdfSummary` (status `pending`) | `400` inválido, `409` duplicado (con `existing_id`), `503` downstream |
| `GET` | `/api/pdfs` | `200` `[PdfSummary]` | — |
| `GET` | `/api/pdfs/{id}` | `200` `PdfDocument` | `404` no existe |
| `DELETE` | `/api/pdfs/{id}` | `204` | `404` no existe |
| `GET` | `/api/pdfs/{id}/download?format=pdf\|markdown` | `200` archivo (`Content-Disposition: attachment; filename="{title}.{ext}"`) | `404`, `409` aún `pending`, `422` si `failed`, `400` formato inválido |

- Subida: `multipart/form-data` con `file` (binario) + `title` (opcional, default = nombre del archivo sin extensión).
- CORS: `allow all` (la futura web lo necesita); se puede ajustar después.

---

## Flujo de subida (async)

```
POST /api/pdfs
  → pdf-validator.Validate (HTTP sync)          → clasifica + checksum  → 400 si inválido
  → pdf-persistance.FindByChecksum (HTTP sync)  → 409 si duplicado
  → pdf-persistance.Create (HTTP sync)          → record status="pending"
  → enqueue:
        pdf      → queue:extraction   (pdf-extractor → pdf-transformator → HTML)
        markdown → queue:conversion   (pdf-converter → HTML)
  → 202 PdfSummary(status=pending)

Después (async):
  pdf-main consume queue:extraction-results / queue:conversion-results
    → pdf-persistance.Update(content_html, status="done" | "failed")
```

## Flujo de descarga (sync)

```
GET /api/pdfs/{id}/download?format=pdf|markdown
  → pdf-persistance.Get(id)            → content_html + title   → 404 si no existe
  → status != "done"                   → 409 (pending) / 422 (failed)
  → pdf-converter.Convert (HTTP sync)  → HTML → pdf|markdown
  → devolver archivo (Content-Disposition)
```

---

## Resiliencia interna (circuit breaker en Go)

Además del middleware de circuit breaker de Traefik (borde público, en `pdf-infra`), `pdf-main` protege sus llamadas internas a downstream con `gobreaker`, uno por cliente HTTP (`validator`, `converter`, `persistance`):

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
| `PERSISTENCE_URL` | `http://pdf-persistance:8000` | Base URL de `pdf-persistance` |
| `CONVERTER_URL` | `http://pdf-converter:8000` | Base URL de `pdf-converter` |
| `REDIS_QUEUE_ADDR` | `redis-queue:6379` | Redis de colas (ADR-0004) |
| `MAX_FILE_SIZE_MB` | `10` | Tope de tamaño heredado del monolito |

**Streams (constantes, ADR-0004):** `queue:extraction`, `queue:extraction-results`, `queue:conversion`, `queue:conversion-results`, con consumer groups y `XACK`.

**Traefik:** `pdf-main` declara en su `docker-compose` los labels de enrutado (`Host(api.pdfmanager.local)`, entrypoint `websecure`, TLS) y referencia los middlewares que define `pdf-infra` (rate-limit y circuit breaker). Ver Open Questions.

---

## Boundaries

- **Always:**
  - `gofmt` + `go test ./...` + `go vet ./...` antes de commit.
  - JSON tags `snake_case` (ADR-0002); binario en base64; texto (HTML/MD) como string.
  - Errores al cliente en RFC 9457 (ADR-0001).
  - Núcleo (`orchestrator`) sin conocimiento de HTTP ni cola (ADR-0004).
  - Todo en RAM: nunca escribir a disco.
  - Contextos con timeout en toda llamada downstream.

- **Ask first:**
  - Cambiar un field name de un DTO (es contrato compartido con otros servicios).
  - Agregar una dependencia de módulo nueva.
  - Cambiar nombres de streams / consumer groups.
  - Cambiar el modelo de persistencia (implica coordinar con `pdf-persistance`).
  - Bump de versión de Go.

- **Never:**
  - Commitear secretos / `.env`.
  - Escribir archivos temporales en disco.
  - Hablar con MongoDB directamente (siempre vía `pdf-persistance`).
  - Hablar con `pdf-extractor` o `pdf-transformator` directamente.
  - Exponer puerto al host (solo accesible vía Traefik).

---

## Success Criteria

1. `POST /api/pdfs` con un PDF válido responde `202` con `PdfSummary` (`status=pending`) y, tras el async, `GET /api/pdfs/{id}` muestra `status=done` con `content_html` poblado.
2. Un PDF con magic bytes inválidos responde `400` RFC 9457 (`title: "Archivo inválido"`).
3. Subir el mismo contenido dos veces responde `409` con `existing_id`.
4. Un Markdown válido pasa por `queue:conversion` (no por extracción) y persiste HTML.
5. `GET /api/pdfs/{id}/download?format=pdf` devuelve `application/pdf`; `format=markdown` devuelve `text/markdown`; ambos con `Content-Disposition`.
6. Descargar un documento aún `pending` → `409`; uno `failed` → `422`.
7. `GET /api/pdfs/{id}` inexistente → `404`; `DELETE` → `204`.
8. Un timeout/5xx de un servicio downstream se traduce a `503` RFC 9457 (no un panic ni un stack trace).
9. `gofmt -l .` no reporta nada; `go test ./...` y `go vet ./...` pasan.
10. Con el breaker en estado abierto, una llamada a un servicio downstream falla rápido con `503` RFC 9457, sin esperar el timeout.

---

## Open Questions

1. **Framework HTTP:** decidí `net/http` stdlib (KISS, sin deps). ¿O preferís `chi`/`gin`?
2. **Circuit breaker:** el middleware de circuit breaker de Traefik lo va a agregar `pdf-infra` (pendiente del usuario, antes del PLAN). `pdf-main` solo lo referencia vía labels en su `docker-compose`; el nombre del middleware se confirma al coordinar con `pdf-infra`.
3. **Detalle de error en `failed`:** hoy `status=failed` no guarda el `error` de downstream en persistencia (solo se loguea). ¿Querés persistir un campo `error` para que la web lo muestre?
4. **`status` en persistencia:** confirmar con `pdf-persistance` que el modelo incluye `status` (ver nota de coordinación arriba).
