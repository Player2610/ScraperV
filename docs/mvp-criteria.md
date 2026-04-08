# Criterios del MVP

El proyecto no tiene deadline. Estos criterios definen cuándo el sistema está listo para recibir órdenes reales.

## Infraestructura

- [ ] Cloud SQL con backups automáticos habilitados y verificados (restaurar desde backup en entorno de test)
- [ ] Cloud Run Service (API) desplegado con rollback probado
- [ ] Cloud Run Job (scraper) desplegado y corriendo por schedule
- [ ] Secretos en Secret Manager, no en variables de entorno planas
- [ ] Alertas de error rate configuradas en Cloud Monitoring

## Scraping

- [ ] Scraper de al menos 2 tiendas funcionando
- [ ] 7 días consecutivos de ejecución nightly sin intervención manual
- [ ] Alerta por email funciona cuando el job falla o retorna <20% del histórico
- [ ] ScrapeRules editables desde DB verificadas (cambiar selector y confirmar que próximo job usa el nuevo)

## Catálogo

- [ ] Listings con `price_on_request` no son visibles al estudiante
- [ ] Listings con `out_of_stock` no son visibles o muestran advertencia clara
- [ ] Búsqueda FTS retorna resultados relevantes para términos comunes ("Arduino", "sensor temperatura", "resistencia 10k")
- [ ] Timestamp "precio actualizado hace X" visible en cada listing

## Usuario y carrito

- [ ] Registro y login funcional
- [ ] Carrito persiste en DB: cerrar browser y volver muestra el mismo carrito
- [ ] Cart guest (sin login) migra a DB al hacer login sin perder ítems
- [ ] Items no disponibles en el carrito muestran advertencia

## Checkout y órdenes

- [ ] Dirección fuera de Bogotá/Soacha rechazada con mensaje claro
- [ ] Tarifa de delivery calculada correctamente para al menos 10 direcciones de prueba en Bogotá
- [ ] Descuento multi-tienda visible en checkout cuando aplica
- [ ] Orden creada con snapshots inmutables: precio, nombre del producto, dirección
- [ ] Email de confirmación llega al estudiante en menos de 2 minutos

## Panel del operador

- [ ] Operador puede completar ciclo completo desde un teléfono: confirmar → purchasing → in_delivery → delivered
- [ ] Cancelación parcial de ítems funciona sin cancelar la orden completa
- [ ] Registro de pago (método + monto) funcional
- [ ] Email enviado al estudiante en cada cambio de estado

## Seguridad y legal

- [ ] HTTPS en todas las rutas
- [ ] Passwords hasheadas (bcrypt)
- [ ] JWT con expiración razonable
- [ ] Política de privacidad publicada (Ley 1581/2012 HABEAS DATA)
- [ ] Consentimiento de datos en registro
- [ ] Canal de solicitud de eliminación de datos operativo
- [ ] OWASP top 10 revisado: SQL injection, XSS, CSRF

## Soft launch

- [ ] 5 órdenes de prueba completas de principio a fin (incluyendo entrega física y pago)
- [ ] Ningún dato de producción perdido en las pruebas
- [ ] Tiempo de respuesta del API < 500ms p95 en condiciones normales
