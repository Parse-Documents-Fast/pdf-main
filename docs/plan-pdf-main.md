# plan.md — pdf-main

## Qué hay que construir
El orquestador: el único servicio que conoce la secuencia completa del flujo (validar → extraer/convertir → persistir, o persistir → convertir en la descarga). En Go, para aprovechar concurrencia real cuando el sistema tenga que atender varias subidas o descargas al mismo tiempo.

## Cómo construirlo, en orden
1. Exponer los endpoints HTTP que hoy expone el monolito (`POST /api/pdfs`, `GET /api/pdfs`, `GET /api/pdfs/{id}`, etc.) — este es el único servicio con ruta pública detrás de `pdf-infra`.
2. Para cada subida: llamar a `pdf-validator`, esperar su respuesta (PDF o Markdown), y según eso decidir a qué servicio llamar después (`pdf-extractor` o `pdf-converter`).
3. Encadenar el resto del flujo de subida hasta persistencia, propagando errores según ADR-0001 (RFC 9457) en cada paso.
4. Para descarga: pedirle el Markdown a `pdf-persistence`; si el cliente pide Markdown, devolverlo tal cual; si pide PDF, pasarlo por `pdf-converter` (ADR-0005).
5. Todos los DTOs entre `pdf-main` y el resto de los servicios siguen ADR-0002 (JSON, binarios en base64).
6. Manejar los casos de error de cada servicio downstream (timeout, 4xx, 5xx) traduciéndolos a una respuesta coherente para el cliente final.

## Dudas a resolver en el /spec
Cómo se comunica con los demás servicios (HTTP directo o cola) — condiciona el diseño interno de este servicio más que el de ningún otro, porque es quien orquesta todas las llamadas.

## Actualización — transporte por servicio (ADR-0004 + ADR-0005)
La llamada a `pdf-extractor` (que ahora también arma el Markdown, absorbió a `pdf-transformator`) deja de ser HTTP síncrono — es un resultado terminal que nadie espera en el mismo request. `pdf-main` publica el job en `queue:extraction` y responde al cliente de inmediato con `status: pending`.

Además del servidor HTTP, `pdf-main` corre un proceso en background consumiendo `queue:extraction-results`. Al recibir un resultado, llama a `pdf-persistence` (HTTP, sin cambios) para actualizar el registro.

**La subida de un archivo Markdown ya no pasa por ninguna cola** (ADR-0005): `pdf-main` valida y persiste directo, síncrono, sin paso de `pending` — no hay ningún cómputo entre validar y guardar.

La comunicación con `pdf-validator` (gatilla la decisión de ruteo del propio `pdf-main`), con `pdf-converter` (solo en descarga a PDF, el cliente espera el archivo en esa misma conexión), y con `pdf-persistence`, sigue siendo HTTP síncrono.
