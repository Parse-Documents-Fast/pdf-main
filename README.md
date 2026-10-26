# pdf-main

Microservicio orquestador de negocio escrito en Go. Su tarea es orquestar los 4 repositorios hermanos (`pdf-validator`, `pdf-extractor`, `pdf-persistence` y `pdf-infra`) en las tareas de extracción y conversión de documentos (PDF a Markdown / Markdown a PDF).

Posee una arquitectura hexagonal para desacoplar completamente la lógica de negocio interna de las interfaces de entrada/salida (HTTP handlers, adaptadores gRPC, clientes TCP y persistencia).

Es el único microservicio que interactúa directamente con `pdf-infra`, siendo el punto de convergencia entre la lógica de orquestación y la infraestructura del sistema (colas en Redis Streams y almacenamiento).

## Características

- **Arquitectura Hexagonal**: clara separación entre capa de dominio, casos de uso y adaptadores de infraestructura.
- **Procesamiento Concurrente Controlado**: control de concurrencia optimizado con goroutines y worker pools para evitar saturación de CPU y RAM bajo carga extrema.
- **Compilado Estático**: construcción como binario único minimalista sin dependencias externas en C (`CGO_ENABLED=0`).
- **Contenedor Seguro**: Dockerfile multi-stage ejecutado bajo un usuario no privilegiado (non-root).
- **Soporte Dual de Procesamiento**: procesamiento síncrono ultra-rápido en RAM para la API de extracción directa, y flujo asíncrono para colas pesadas.

## Arquitectura y Contratos

`pdf-main` actúa como API Gateway interno y orquestador:

- **`pdf-validator`**: valida la estructura, permisos y corrupción del PDF binario antes del procesamiento.
- **`pdf-extractor`**: realiza la lectura del stream binario y la extracción de texto a Markdown en memoria.
- **`pdf-persistence`**: consulta y almacena estados del documento, metadatos y caché de deduplicación (`FindByChecksum`).
- **`pdf-infra`**: gestiona el encolado mediante Redis Streams (`redis-queue`) para tareas asíncronas, y el caché en RAM (`redis-cache`).
- **`pdf-converter`**: hace la operación inversa convirtiendo un markdown a PDF usando padoc.

Todos los errores HTTP expuestos por este microservicio cumplen con el estándar RFC 9457 (Problem Details for HTTP APIs).

## Endpoints Principales

### 1. Extracción Síncrona - `POST /extract`

Endpoint principal requerido para evaluación de rendimiento. Diseñado para procesar 100% en RAM sin I/O a disco.

- **Content-Type**: `multipart/form-data` (campo `file`) o binario directo (`application/pdf`).

**Respuesta (`200 OK`):**

```json
{
  "content": "Texto extraído del documento en formato Markdown...",
  "page_count": 292
}
```

### 2. Creación Asíncrona - `POST /api/pdfs`

Permite registrar un PDF en el sistema para su procesamiento asíncrono vía colas de mensajes.

- **Respuesta (`200 Accepted`)**: retorna un objeto `PdfSummary` con el estado `pending` y el identificador asignado.

## Variables de Entorno

El microservicio se configura mediante variables de entorno (principios Twelve-Factor App):

| Variable | Descripción | Default |
|---|---|---|
| `HTTP_ADDR` | Puerto interno de escucha (detrás de Traefik, no expuesto) | `:8000` |
| `VALIDATOR_URL` | Base URL de `pdf-validator` | `http://pdf-validator:8000` |
| `PERSISTENCE_URL` | Base URL de `pdf-persistence` | `http://pdf-persistence:8000` |
| `CONVERTER_URL` | Base URL de `pdf-converter` | `http://pdf-converter:8000` |
| `EXTRACTOR_URL` | Base URL de `pdf-extractor` (solo `POST /extract`) | `http://pdf-extractor:8000` |
| `REDIS_QUEUE_ADDR` | Dirección de la instancia Redis para colas | `redis-queue:6379` |
| `REDIS_STREAM_EXTRACTION` | Stream de entrada de trabajos de extracción | `queue:extraction` |
| `REDIS_STREAM_EXTRACTION_RESULTS` | Stream de resultados de extracción | `queue:extraction-results` |
| `REDIS_CONSUMER_GROUP` | Grupo de consumo de Redis Streams | `pdf-main` |
| `CONSUMER_NAME` | Nombre del consumidor (único por réplica; por defecto usa el hostname) | auto |
| `MAX_FILE_SIZE_MB` | Tope de tamaño de subida en MB | `20` |
| `LOG_LEVEL` | Nivel de log: `debug`, `info`, `warn` o `error` | `info` |

`GOMAXPROCS` (límite de hilos del runtime de Go) y `PDF_MAIN_REPLICAS` (cantidad de réplicas) se definen a nivel de `docker-compose`, no en el binario.

Los logs siguen el estándar ADR-0007: una línea JSON por evento a stdout, con los campos `time`, `level`, `service`, `msg` y los campos adicionales definidos en el ADR. Se filtran con `docker compose logs | jq`.

## Ejecución en Docker

### 1. Entorno normal / desarrollo

En condiciones normales de desarrollo local, la infraestructura se levanta en la red interna `fast_pdf_network` detrás del reverse proxy Traefik:

```bash
docker compose up -d --build
```

### 2. Entorno de benchmark y pruebas de estrés (TP UTN)

Para ejecutar el benchmark en igualdad de condiciones según la consigna de la cátedra (límites estrictos de hardware de 1.0 CPU y 1 GB RAM por instancia, escalado a 5 réplicas y ruteo HTTP transparente):

```bash
docker compose -f docker-compose.yml -f docker-compose.tp.yml up -d --build
```

> **Nota:** la configuración `docker-compose.tp.yml` deshabilita el Rate Limiting y el Circuit Breaker del API Gateway, fija `GOMAXPROCS=1` y habilita el entrypoint directo en el puerto 80 para ser atacado por k6 y Vegeta. Utilizar únicamente para propósitos de pruebas de carga.

## Pruebas de Carga (Benchmarking)

Para reproducir los benchmarks oficiales contra el endpoint `/extract`:

**Prueba Spike (modelo cerrado - Grafana k6):**

```bash
k6 run --vus 100 --duration 40s tests/stress/k6-spike.js
```

**Prueba de Throughput Constante (modelo abierto - Vegeta):**

```bash
vegeta attack -rate=50/s -duration=30s -targets=tests/stress/targets.txt | vegeta report
```