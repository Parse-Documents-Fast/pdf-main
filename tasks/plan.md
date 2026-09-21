# Implementation Plan: pdf-main

## Overview

Construir `pdf-main`, el orquestador de la lógica de negocio del sistema *Parse Documents Fast*, en Go. Expone la API HTTP pública detrás de Traefik, valida/encola/consulta/descarga documentos coordinando a `pdf-validator`, `pdf-extractor`, `pdf-converter` y `pdf-persistance`, con resiliencia interna (timeouts + circuit breaker). Todo en RAM, errores en RFC 9457, DTOs en JSON `snake_case` con binario en base64.

**Seguimiento:** las tareas de abajo son la fuente de las issues. El usuario las mapea 1:1 a GitHub Issues, agrupadas en **4 milestones, uno por fase** (milestones pequeñas).

## Architecture Decisions

1. **Router `chi/v5`** — aprobado en spec. Middlewares (CORS, logging) y path params ergonómicos.
2. **Ports/interfaces en el consumidor (`orchestrator`)** — el núcleo define las interfaces (`Validator`, `Persistence`, `Converter`, `Queue`); `clients/*` y `queue/*` las implementan. Refinamiento menor del snippet ilustrativo del spec (que mostraba `clients.Validator`): mantener interfaces en `orchestrator/ports.go` evita acoplar el núcleo a los adaptadores.
3. **Stubs de servicios en `test/`** — como este es el primer repo, los servicios downstream aún no existen. `test/stubs` levanta servidores `httptest` falsos de validator/persistance/converter con respuestas configurables, para probar clientes y orquestador sin dependencias reales. El productor de cola se testea contra un `Queue` fake en memoria; el consumidor se separa en un adapter Redis delgado + una función pura de manejo de resultado.
4. **Circuit breaker con `gobreaker`** — un breaker por cliente HTTP interno. En abierto → `503` inmediato. Testeable forzando el estado abierto (no depender de timers).
5. **Consumidores de resultados** — la lógica de "recibir resultado → actualizar persistencia" es una función pura testeable; el loop de Redis Streams (`XREADGROUP`/`XACK`) queda como capa delgada sin cobertura (se valida en integración manual).
6. **Deploy en `docker-compose.yml`** — labels de Traefik (`rate-limit-redis@file,cb-documents@file`) y `networks: [fast_pdf_network]`; la imagen compila solo `cmd/pdf-main` (los stubs de `test/` nunca entran al binario).

## Task List

### Milestone 1 — Foundation
- [ ] Task 1: Bootstrap del módulo Go + config desde env
- [ ] Task 2: DTOs del wire + helpers RFC 9457
- [ ] Task 3: Stubs de servicios en `test/`

### Checkpoint: Foundation
- [ ] `go build ./...` limpio; `go vet ./...` sin errores
- [ ] Los DTOs compilan con tags `snake_case` correctos
- [ ] Los stubs responden JSON según los DTOs del spec

### Milestone 2 — Subida (slice vertical)
- [ ] Task 4: Ports + clientes HTTP con circuit breaker
- [ ] Task 5: Núcleo de subida (`Submit`)
- [ ] Task 6: Producer de cola (+ fake en memoria)
- [ ] Task 7: Handler `POST /api/pdfs` + router chi

### Checkpoint: Subida
- [ ] POST válido → 202 `pending`; inválido → 400; duplicado → 409 (todo con stubs)

### Milestone 3 — CRUD + descarga + async
- [ ] Task 8: Núcleo + handlers de query/list/get/delete
- [ ] Task 9: Descarga (núcleo + handler)
- [ ] Task 10: Consumers de resultados → actualizar persistencia

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
| Directorios `dev/` y `test/` vacíos (sobrantes de template) | Low | `dev/` ya fue borrado; `test/` se usa para stubs |

## Open Questions

- _Ninguna pendiente._
