# Tasks — pdf-main

> Tareas ordenadas por dependencia, agrupadas en **4 milestones (uno por fase)**. Cada tarea mapea a una issue.

## Milestone 1 — Foundation

## Task 1: Bootstrap del módulo Go + config desde env

**Description:** Inicializar el módulo Go (`github.com/Parse-Documents-Fast/pdf-main`), fijar dependencias (`chi/v5`, `go-redis/v9`, `gobreaker`) y crear el loader de configuración por variables de entorno.

**Acceptance criteria:**
- [ ] `go.mod` con módulo `github.com/Parse-Documents-Fast/pdf-main` y las 3 dependencias en `go.sum`
- [ ] `internal/config` lee `HTTP_ADDR`, `VALIDATOR_URL`, `PERSISTENCE_URL`, `CONVERTER_URL`, `REDIS_QUEUE_ADDR`, `MAX_FILE_SIZE_MB` con defaults del spec
- [ ] Config expuesta como struct inmutable con un `Load()` testeable

**Verification:**
- [ ] `go build ./...` y `go vet ./...` pasan
- [ ] Test unitario de `Load()` con env y defaults

**Dependencies:** None

**Files likely touched:**
- `go.mod`, `go.sum`
- `internal/config/config.go`
- `internal/config/config_test.go`

**Estimated scope:** Small (2-3 files)

---

## Task 2: DTOs del wire + helpers RFC 9457

**Description:** Definir todos los DTOs del contrato (público, validator, extractor, converter, persistance) con tags `snake_case`, y los helpers de Problem Details (RFC 9457) con `Content-Type: application/problem+json`.

**Acceptance criteria:**
- [ ] `internal/dto` con structs para `PdfSummary`, `PdfDocument`, y los payloads de validator/extractor/converter/persistance según `docs/spec.md`
- [ ] Campos binarios tipados como `[]byte` con marshaling a base64 (o `string` para texto); tags `snake_case`
- [ ] `internal/problem` con `WriteProblem(w, status, title, detail, instance)` y extensiones (`existing_id`)
- [ ] Test de marshal/unmarshal de cada DTO contra el JSON de ejemplo del spec

**Verification:**
- [ ] `go test ./internal/dto/... ./internal/problem/...`
- [ ] El JSON producido coincide field por field con los ejemplos del spec

**Dependencies:** Task 1

**Files likely touched:**
- `internal/dto/documents.go`, `internal/dto/validator.go`, `internal/dto/extractor.go`, `internal/dto/converter.go`
- `internal/problem/problem.go` + tests

**Estimated scope:** Medium (5-6 files)

---

## Task 3: Stubs de servicios en `test/`

**Description:** Crear el paquete `test/stubs` con servidores `httptest` falsos para `pdf-validator`, `pdf-persistance` y `pdf-converter`, con respuestas configurables, para poder armar la lógica de negocio sin servicios reales (este es el primer repo).

**Acceptance criteria:**
- [ ] `test/stubs` con stub de validator (devuelve `original_format` + `checksum`, o error 400)
- [ ] Stub de persistance en memoria (create/get/findByChecksum/list/update/delete con estado)
- [ ] Stub de converter (ingesta y descarga; devuelve `content_base64` + `mime_type`)
- [ ] Cada stub expone una URL (`httptest`) y permite inyectar fallos/timeouts
- [ ] `Queue` fake en memoria (producer/consumer) para tests de orquestación

**Verification:**
- [ ] `go build ./...` compila (el paquete no lo importa `main`, no entra al binario)
- [ ] Test de smoke de cada stub (responde el JSON esperado)

**Dependencies:** Task 2

**Files likely touched:**
- `test/stubs/validator_stub.go`, `test/stubs/persistance_stub.go`, `test/stubs/converter_stub.go`
- `test/stubs/memory_queue.go`, `test/stubs/stubs_test.go`

**Estimated scope:** Medium (4-5 files)

---

## Milestone 2 — Subida (slice vertical)

## Task 4: Ports + clientes HTTP con circuit breaker

**Description:** Definir las interfaces de puerto (`Validator`, `Persistence`, `Converter`, `Queue`) y sus implementaciones HTTP (`clients/*`) wrappeadas con `gobreaker` y timeouts.

**Acceptance criteria:**
- [ ] `internal/orchestrator/ports.go` con las interfaces que consume el núcleo
- [ ] `internal/clients` implementa validator/persistance/converter con `net/http` + base64 decode/encode
- [ ] Cada cliente envuelto en un `gobreaker.CircuitBreaker`; estado abierto → error `ErrDownstream`
- [ ] Errores de red/timeout/5xx mapeados a `ErrDownstream`; 4xx de negocio (404/409) preservados como errores de dominio

**Verification:**
- [ ] `go test ./internal/clients/...` con `test/stubs` (incluye caso de breaker abierto forzado)
- [ ] `go vet ./...`

**Dependencies:** Task 2, Task 3

**Files likely touched:**
- `internal/orchestrator/ports.go`
- `internal/clients/validator.go`, `internal/clients/persistance.go`, `internal/clients/converter.go`
- `internal/clients/clients_test.go`

**Estimated scope:** Medium (4-5 files)

---

## Task 5: Núcleo de subida (`Submit`)

**Description:** Implementar la función pura de orquestación de subida: validar → detectar duplicado → crear `pending` → encolar según formato.

**Acceptance criteria:**
- [ ] `Submit` devuelve `PdfSummary` (`status=pending`) y errores de dominio (`ErrInvalid`, `ErrDuplicate`, `ErrDownstream`)
- [ ] PDF → `PublishExtraction`; Markdown → `PublishConversion` (según `original_format`)
- [ ] Duplicado detectado por `FindByChecksum` → `ErrDuplicate` con `existing_id`
- [ ] Sin escritura a disco; contenido en memoria

**Verification:**
- [ ] `go test ./internal/orchestrator/...` con `test/stubs` + `memory_queue` (casos: pdf, markdown, duplicado, downstream falla)

**Dependencies:** Task 4, Task 3

**Files likely touched:**
- `internal/orchestrator/upload.go`
- `internal/orchestrator/upload_test.go`

**Estimated scope:** Small (2 files)

---

## Task 6: Producer de cola (+ fake en memoria)

**Description:** Implementar el productor de cola Redis Streams (`XADD`) para `queue:extraction` y `queue:conversion`, con una implementación en memoria para tests.

**Acceptance criteria:**
- [ ] `queue.Producer` con `PublishExtraction` y `PublishConversion` escribiendo los DTOs del spec como JSON
- [ ] `XADD` con el job en los campos correctos (base64 para PDF, string para Markdown)
- [ ] `memory_queue` ya provisto en Task 3 satisface la misma interfaz

**Verification:**
- [ ] `go test ./internal/queue/...` (fake en memoria); el adapter Redis se deja sin cobertura (integración manual)

**Dependencies:** Task 4 (ports), Task 3 (fake)

**Files likely touched:**
- `internal/queue/producer.go`
- `internal/queue/producer_test.go`

**Estimated scope:** Small (2 files)

---

## Task 7: Handler `POST /api/pdfs` + router chi

**Description:** Montar el router chi y el handler de subida que traduce el `multipart/form-data` al núcleo y mapea errores de dominio a RFC 9457.

**Acceptance criteria:**
- [ ] Router chi con la ruta `POST /api/pdfs` y middleware CORS
- [ ] Lee `file` + `title` (default = nombre sin extensión) y llama al núcleo
- [ ] Mapeo: `ErrInvalid`→400, `ErrDuplicate`→409 (+`existing_id`), `ErrDownstream`→503; éxito→202 con `PdfSummary`
- [ ] Respuestas de error en RFC 9457 (`application/problem+json`)

**Verification:**
- [ ] `go test ./internal/httpapi/...` con `httptest` + `test/stubs` (202/400/409/503)

**Dependencies:** Task 5, Task 6

**Files likely touched:**
- `internal/httpapi/router.go`
- `internal/httpapi/handler_upload.go`
- `internal/httpapi/handler_upload_test.go`

**Estimated scope:** Medium (3 files)

---

## Milestone 3 — CRUD + descarga + async

## Task 8: Núcleo + handlers de query/list/get/delete

**Description:** Implementar las operaciones de consulta del núcleo (list, get by id, delete) y sus handlers.

**Acceptance criteria:**
- [ ] Núcleo `List`, `Get`, `Delete` delegando en `Persistence`
- [ ] Handlers `GET /api/pdfs`, `GET /api/pdfs/{id}`, `DELETE /api/pdfs/{id}`
- [ ] `Get` devuelve `PdfDocument` (con `content_html` y `error`); no existe → 404 RFC 9457
- [ ] `Delete` → 204; no existe → 404

**Verification:**
- [ ] `go test ./internal/orchestrator/... ./internal/httpapi/...` (200/404/204 con stubs)

**Dependencies:** Task 7

**Files likely touched:**
- `internal/orchestrator/query.go`, `internal/orchestrator/delete.go`
- `internal/httpapi/handler_query.go` + tests

**Estimated scope:** Medium (3-4 files)

---

## Task 9: Descarga (núcleo + handler)

**Description:** Implementar la orquestación de descarga: obtener HTML → convertir a `pdf|markdown` → devolver archivo.

**Acceptance criteria:**
- [ ] Núcleo `Download(id, format)` valida formato, obtiene documento, y llama a `Converter.Convert` (HTML → target)
- [ ] `status != done` → `ErrNotReady` (409 si pending) / `ErrFailed` (422 si failed)
- [ ] Handler `GET /api/pdfs/{id}/download?format=pdf|markdown` con `Content-Disposition` y `Content-Type` correctos
- [ ] Formato inválido → 400

**Verification:**
- [ ] `go test ./internal/orchestrator/... ./internal/httpapi/...` (pdf/markdown, 404, 409, 422, 400)

**Dependencies:** Task 8

**Files likely touched:**
- `internal/orchestrator/download.go`
- `internal/httpapi/handler_download.go` + tests

**Estimated scope:** Small (2-3 files)

---

## Task 10: Consumers de resultados → actualizar persistencia

**Description:** Implementar los consumidores de `queue:extraction-results` y `queue:conversion-results` que actualizan la persistencia con `content_html` + `status` (`done`/`failed`), con una función pura de manejo testeable.

**Acceptance criteria:**
- [ ] Función pura `HandleResult(result, persistence)` que mapea un resultado a `Persistence.Update`
- [ ] Resultado `done` → `{content_html, status:"done"}`; `failed` → `{status:"failed", error:<detail>}`
- [ ] Adapter Redis con consumer groups (`XREADGROUP`/`XACK`) delgado, sin lógica de negocio
- [ ] El `error` persistido es el `detail` (string) del RFC 9457 de downstream

**Verification:**
- [ ] `go test ./internal/queue/...` (función pura con fake persistence); adapter Redis sin cobertura

**Dependencies:** Task 6, Task 8

**Files likely touched:**
- `internal/queue/consumer.go` (+ handler puro)
- `internal/queue/consumer_test.go`

**Estimated scope:** Medium (2-3 files)

---

## Milestone 4 — Wiring + deploy

## Task 11: `main.go` (wiring + graceful shutdown)

**Description:** Ensamblar config, clientes, núcleo, productor, consumidores y server HTTP; graceful shutdown de server + consumidores.

**Acceptance criteria:**
- [ ] `cmd/pdf-main/main.go` arma el grafo de dependencias y arranca server + goroutines de consumidores
- [ ] Señales (SIGINT/SIGTERM) → shutdown limpio (context cancel, server + consumers)
- [ ] El binario no importa `test/stubs`

**Verification:**
- [ ] `go build ./cmd/pdf-main` y `go run` arranca sin panics (sin servicios reales: arranca y queda esperando)

**Dependencies:** Task 7, Task 9, Task 10

**Files likely touched:**
- `cmd/pdf-main/main.go`

**Estimated scope:** Small (1 file)

---

## Task 12: `Dockerfile` + `docker-compose.yml` (labels Traefik)

**Description:** Crear imagen multi-stage de Go y el `docker-compose.yml` con labels de Traefik y red compartida.

**Acceptance criteria:**
- [ ] `Dockerfile` multi-stage (build → imagen final sin toolchain, usuario no-root)
- [ ] `docker-compose.yml` con `traefik.enable=true`, router `pdf-main`, entrypoint `websecure`, TLS, middlewares `rate-limit-redis@file,cb-documents@file`
- [ ] `networks: [fast_pdf_network]` (red de `pdf-infra`)
- [ ] Sin `ports` expuestos al host

**Verification:**
- [ ] `docker build .` compila la imagen
- [ ] `docker compose config` valida el compose

**Dependencies:** Task 11

**Files likely touched:**
- `Dockerfile`
- `docker-compose.yml`

**Estimated scope:** Small (2 files)

---

## Checkpoints

### Checkpoint: Milestone 1 — Foundation (tras Tasks 1-3)
- [ ] `go build ./...` y `go vet ./...` limpios
- [ ] DTOs y stubs alineados con el spec

### Checkpoint: Milestone 2 — Subida (tras Tasks 4-7)
- [ ] POST funciona end-to-end con stubs: 202 / 400 / 409 / 503

### Checkpoint: Milestone 3 — Flujos completos (tras Tasks 8-10)
- [ ] CRUD + descarga + consumidores verificados con stubs

### Checkpoint: Milestone 4 — Complete (tras Tasks 11-12)
- [ ] `gofmt -l .` vacío; `go test ./...` y `go vet ./...` pasan
- [ ] Todos los success criteria del spec cubiertos
