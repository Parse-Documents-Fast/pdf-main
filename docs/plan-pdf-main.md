# plan.md — pdf-main

## Qué hay que construir
El orquestador: el único servicio que conoce la secuencia completa del flujo (validar → extraer/convertir → persistir, o persistir → convertir en la descarga). En Go, para aprovechar concurrencia real cuando el sistema tenga que atender varias subidas o descargas al mismo tiempo.

## Cómo construirlo, en orden
1. Exponer los endpoints HTTP que hoy expone el monolito (`POST /api/pdfs`, `GET /api/pdfs`, `GET /api/pdfs/{id}`, etc.) — este es el único servicio con ruta pública detrás de `pdf-infra`.
2. Para cada subida: llamar a `pdf-validator`, esperar su respuesta (PDF o Markdown), y según eso decidir a qué servicio llamar después (`pdf-extractor` o `pdf-converter`).
3. Encadenar el resto del flujo de subida hasta persistencia, propagando errores según ADR-0001 (RFC 9457) en cada paso.
4. Para descarga: pedirle el HTML a `pdf-persistance`, pasarlo por `pdf-converter` con el formato que pidió el cliente, devolver la respuesta.
5. Todos los DTOs entre `pdf-main` y el resto de los servicios siguen ADR-0002 (JSON, binarios en base64).
6. Manejar los casos de error de cada servicio downstream (timeout, 4xx, 5xx) traduciéndolos a una respuesta coherente para el cliente final. (resilencia interna)
7. Circuit Breaker con los labels de Traefik.

## Dudas a resolver en el /spec
Cómo se comunica con los demás servicios (HTTP directo o cola) — condiciona el diseño interno de este servicio más que el de ningún otro, porque es quien orquesta todas las llamadas.
