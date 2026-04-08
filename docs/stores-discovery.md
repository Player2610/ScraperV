# Discovery de Tiendas

Mapa de todas las tiendas de Bogotá y alrededores con catálogo online de componentes electrónicos.

---

## Tiendas identificadas

| Tienda | URL base | URL catálogo | Rendering | Moneda | Particularidades |
|--------|----------|-------------|-----------|--------|-----------------|
| Sigmaelectrónica | sigmaelectronica.net | `/categoria-producto/componentes-discretos/page/{page}/` | HTML estático (tema Kallyas sobre WooCommerce) | COP | Selectores custom (ver tabla abajo); paginación `a.pagination-item-next-link`; imagen lazy con `data-echo`; `delay_ms: 2000` |
| Electronilab | electronilab.co | /tienda | **Algolia InstantSearch (JS)** | COP | ⛔ No scrapeble con goquery — productos cargados vía JS. Requiere cliente Algolia REST API |
| Vistronica | vistronica.com.co | /catalogo | — | COP | ⛔ Dominio `www.vistronica.com.co` no resuelve en DNS (tienda aparentemente cerrada/migrada) |
| I+D Electrónica | idelectronica.co | /tienda | HTML estático (WooCommerce) | COP | Tienda pequeña especializada; catálogo completo; `delay_ms: 1500` |
| Todo en Electrónica | todoelectronica.com | /categoria/componentes | HTML estático | COP | Catálogo amplio; paginación por query param `?paged=N`; algunos precios en USD marcados `price_on_request` |
| Suconel | suconel.com | /productos | HTML estático (custom) | COP | Distribuidor mayorista; precios públicos sin login; paginación por número de página |
| Ferretrónica | ferretronica.com | /tienda | HTML estático (WooCommerce) | COP | Variedad media; stock confiable; `delay_ms: 2000` |
| Naylampmechatronics | naylampmechatronics.com | /Electronica | HTML estático | COP/USD | Sede en Perú; envíos a Colombia; precios en PEN/USD → marcar `price_on_request` hasta implementar conversión |
| JC Electrónica | jcelectronica.com | /tienda | HTML estático (WooCommerce) | COP | Tienda bogotana mediana; foco en módulos Arduino/ESP; estructura WooCommerce estándar |

---

## Resumen de selectores CSS por plataforma

### WooCommerce estándar (I+D, JC Electrónica, futuras tiendas WooCommerce)

| Campo | Selector CSS |
|-------|-------------|
| Item contenedor | `ul.products li.product` |
| Nombre producto | `h2.woocommerce-loop-product__title` |
| Precio | `span.woocommerce-Price-amount bdi` |
| Imagen | `img.wp-post-image` |
| URL producto | `a.woocommerce-LoopProduct-link` (atributo `href`) |
| SKU | No disponible en listing; generar hash |
| Stock (agotado) | `.button.disabled`, `.out-of-stock`, texto "Agotado" |
| Paginación | `a.next.page-numbers` |

### Sigmaelectrónica (tema Kallyas — selectores reales verificados 2026-04-08)

| Campo | Selector CSS | Nota |
|-------|-------------|------|
| Item contenedor | `ul.products li.product` | `<ul class="products columns-4">` — CSS selector funciona |
| Nombre producto | `h3.kw-details-title` | El tema no usa `h2.woocommerce-loop-product__title` |
| Precio | `span.woocommerce-Price-amount bdi` | Igual a WooCommerce estándar |
| Imagen | `img.kw-prodimage-img-secondary` | La imagen primaria usa `data-echo` (lazy load); la secundaria tiene `src` directo |
| URL producto | `a.woocommerce-LoopProduct-link` | — |
| Paginación | `a.pagination-item-next-link` | No usa `a.next.page-numbers` |
| URL catálogo pag. N | `.../page/{page}/` | `/page/1/` hace 301 a la URL base — Go sigue el redirect automáticamente |

### Notas de particularidades por tienda

- **Sigmaelectrónica**: precio puede aparecer como rango (`$1.000 – $2.000`); `ParsePrice` toma el primer valor. IVA puede aparecer como `+ IVA` en el texto; el parser lo stripea.
- **Electronilab**: usa Algolia InstantSearch. `apiKey` visible en el HTML de la página (`Bc35SyX0...`). Para reactivar: implementar cliente REST de Algolia en lugar de goquery.
- **Vistronica**: dominio `www.vistronica.com.co` no resuelve en DNS. Verificar si migró a nueva URL antes de reimplementar.
- **Suconel**: estructura HTML custom, no WooCommerce. Requiere inspección manual.
- **Naylampmechatronics**: excluir del MVP — precios no en COP, logística no confirmada para Bogotá.
- **Todo en Electrónica**: verificar URL base; scrapear solo categorías de componentes.

---

## Prioridad de implementación

| Fase | Tiendas |
|------|---------|
| Fase 1 MVP | Sigmaelectrónica, Electronilab, Vistronica |
| Fase 1 ampliada | I+D Electrónica, JC Electrónica |
| Fase 2 | Suconel, Ferretrónica, Todo en Electrónica |
| Descartadas MVP | Naylampmechatronics (precio no COP) |

---

## Criterio de inclusión

Una tienda se incluye si:
- Tiene catálogo de componentes electrónicos (resistencias, sensores, microcontroladores, etc.)
- Los precios son visibles sin necesidad de cuenta
- La tienda tiene presencia física o logística en Bogotá/alrededores

Una tienda se excluye si:
- Solo vende dispositivos terminados (laptops, cargadores, etc.)
- Requiere login para ver precios
- Está fuera del área de cobertura sin punto de despacho en Bogotá

---

## Checklist de evaluación para nuevas tiendas

1. ¿Tiene catálogo navegable con precios visibles sin login?
2. ¿Usa JavaScript rendering? (revisar con JS desactivado en browser)
3. ¿Tiene paginación? ¿Cómo funciona (query param, infinite scroll, etc.)?
4. ¿Los precios incluyen IVA?
5. ¿Hay productos con "consultar precio"?
6. ¿Tiene anti-bot básico (Cloudflare challenge, rate limiting)?
7. ¿Tiene punto de despacho o cobertura confirmada en Bogotá?
