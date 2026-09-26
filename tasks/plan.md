# Implementation Plan: pdf-main

## Overview

Construir `pdf-main`, el orquestador de la lógica de negocio del sistema *Parse Documents Fast*, en Go. Expone la API HTTP pública detrás de Traefik y coordina a `pdf-validator`, `pdf-extractor`, `pdf-converter` y `pdf-persistence`. El **formato canónico es Markdown** (ADR-0005): la subida de PDF es asíncrona (`pending` → extracción a Markdown) y la de Markdown es síncrona (persiste directo). La descarga devuelve Markdown tal cual o PDF vía converter. Todo en RAM, errores en RFC 9457, DTOs JSON `snake_case` con binario en base64.

**Seguimiento:** las tareas de abajo son la fuente de las issues. El usuario las mapea 1:1 a GitHub Issues, agrupadas en **4 milestones, uno por fase** (milestones pequeñas).

## Architecture Decisions

1. **Router `chi/v5`** — aprobado en spec. Middlewares (CORS, logging) y path params ergonómicos.
2. **Ports/interfaces en el consumidor (`orchestrator`)** — el núcleo define las interfaces (`Validator`, `Persistence`, `Converter`, `Queue`); `clients/*` y `queue/*` las implementan. Refinamiento menor del snippet ilustrativo del spec: mantener interfaces en `orchestrator/ports.go` evita acoplar el núcleo a los adaptadores.
3. **Stubs de servicios en `test/`** — como este es el primer repo, los servicios downstream aún no existen. `test/stubs` levanta servidores `httptest` falsos de validator/persistence/converter (este último **solo descarga**, Markdown→PDF) y un `Queue` fake en memoria para el flujo de extracción. Los stubs nunca entran al binario.
4. **Circuit breaker con `gobreaker`** — un breaker por cliente HTTP interno (`validator`, `converter`, `persistence`). En abierto → `503` inmediato. Testeable forzando el estado abierto.
5. **Un solo consumer de resultados** (`queue:extraction-results`) — ADR-0005 elimina `queue:conversion*`. La lógica "recibir resultado → actualizar persistencia" es una función pura testeable; el loop de Redis Streams queda como capa delgada sin cobertura.
6. **Deploy en `docker-compose.yml`** — labels de Traefik (`rate-limit-redis@file,cb-documents@file`) y `networks: [fast_pdf_network]`; la imagen compila solo `cmd/pdf-main`.

## Task List

### Milestone 1 — Foundation
- [ ] Task 1: Bootstrap del módulo Go + config desde env
- [ ] Task 2: DTOs del wire + helpers RFC 9457
- [ ] Task 3: Stubs de servicios en `test/`

### Checkpoint: Foundation
- [ ] `go build ./...` limpio; `go vet ./...` sin errores
- [ ] Los DTOs compilan con tags `snake_case` correctos (incluye `content` Markdown)
- [ ] Los stubs responden JSON según los DTOs del spec

### Milestone 2 — Subida (slice vertical)
- [ ] Task 4: Ports + clientes HTTP con circuit breaker
- [ ] Task 5: Núcleo de subida (`Submit`, dos caminos pdf/markdown)
- [ ] Task 6: Producer de cola (solo extracción) + fake en memoria
- [ ] Task 7: Handler `POST /api/pdfs` + router chi

### Checkpoint: Subida
- [ ] POST → 200 (PDF `pending` / Markdown `done`); inválido → 400; duplicado → 409 (con stubs)

### Milestone 3 — CRUD + descarga + async
- [ ] Task 8: Núcleo + handlers de query/list/get/delete
- [ ] Task 9: Descarga (núcleo + handler: markdown passthrough / pdf vía converter)
- [ ] Task 10: Consumer de `queue:extraction-results` → actualizar persistencia

### Checkpoint: Flujos completos
- [ ] GET/DELETE/list/descarga funcionan contra stubs; consumidor actualiza `done`/`failed`

### Milestone 4 — Wiring + deploy
- [ ] Task 11: `main.go` (wiring + graceful shutdown)
- [ ] Task 12: `Dockerfile` + `docker-compose.yml` (labels Traefik)

### Checkpoint: Complete
- [ ] Todos los success criteria del spec verificados
- [ ] `gofmt -l .` vacío; `go test ./...` y `go vet ./...` pasan
- [ ] Listo para REVIEW

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Redis Streams (consumer groups, XACK) no testeable sin Redis real | High | Consumer como adapter delgado + función pura de manejo; validación manual contra `pdf-infra` más adelante |
| Drift de DTOs con servicios downstream que aún no existen | High | Los DTOs son fuente de verdad en `docs/spec.md`; los stubs los codifican; coordinar con los otros repos al tocarlos |
| Tests de circuit breaker dependientes de timers → flaky | Med | Forzar estado abierto del breaker en test (inyectar breaker ya abierto), no esperar timers |
| Cambios recientes de requisitos (ADR-0005) | Med | Spec actualizado y aprobado; plan refleja Markdown canónico, un solo stream y converter solo-descarga |

## Open Questions

- _Ninguna pendiente._
