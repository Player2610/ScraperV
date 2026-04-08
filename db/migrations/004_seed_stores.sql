-- +goose Up

-- +goose StatementBegin
-- Seed stores: Sigmaelectrónica, Electronilab, Vistronica
-- Coordinates are approximate central Bogotá locations per store zone.
INSERT INTO stores (name, base_url, lat, lng, is_active) VALUES
  ('Sigmaelectrónica', 'https://www.sigmaelectronica.net',  4.628900, -74.064800, true),
  ('Electronilab',     'https://electronilab.co',           4.710400, -74.072100, true),
  ('Vistronica',       'https://www.vistronica.com.co',     4.601200, -74.076300, true);
-- +goose StatementEnd

-- +goose StatementBegin
-- Scrape rules — WooCommerce selectors common to all three stores.
-- Selectors will be refined after live testing in Task 1.1 fieldwork.
INSERT INTO scrape_rules (
  store_id,
  catalog_url_pattern,
  item_selector,
  price_selector,
  name_selector,
  image_selector,
  stock_selector,
  sku_selector,
  pagination_selector,
  headers_json,
  delay_ms
)
SELECT
  s.id,
  rule.catalog_url_pattern,
  rule.item_selector,
  rule.price_selector,
  rule.name_selector,
  rule.image_selector,
  rule.stock_selector,
  rule.sku_selector,
  rule.pagination_selector,
  rule.headers_json,
  rule.delay_ms
FROM stores s
JOIN (VALUES
  (
    'Sigmaelectrónica',
    'https://www.sigmaelectronica.net/categoria-producto/componentes-discretos/page/{page}/',
    'ul.products li.product',
    'span.woocommerce-Price-amount bdi',
    'h2.woocommerce-loop-product__title',
    'img.wp-post-image',
    'span.out-of-stock, .button.disabled',
    NULL,
    'a.next.page-numbers',
    '{"Accept-Language": "es-CO,es;q=0.9", "Accept": "text/html"}'::jsonb,
    2000
  ),
  (
    'Electronilab',
    'https://electronilab.co/tienda/page/{page}/',
    'ul.products li.product',
    'span.woocommerce-Price-amount bdi',
    'h2.woocommerce-loop-product__title',
    'img.wp-post-image',
    'span.out-of-stock',
    NULL,
    'a.next.page-numbers',
    '{"Accept-Language": "es-CO,es;q=0.9", "Accept": "text/html"}'::jsonb,
    1500
  ),
  (
    'Vistronica',
    'https://www.vistronica.com.co/catalogo/page/{page}/',
    'ul.products li.product',
    'span.woocommerce-Price-amount bdi',
    'h2.woocommerce-loop-product__title',
    'img.wp-post-image',
    'span.out-of-stock',
    NULL,
    'a.next.page-numbers',
    '{"Accept-Language": "es-CO,es;q=0.9", "Accept": "text/html"}'::jsonb,
    2000
  )
) AS rule(
  store_name,
  catalog_url_pattern,
  item_selector,
  price_selector,
  name_selector,
  image_selector,
  stock_selector,
  sku_selector,
  pagination_selector,
  headers_json,
  delay_ms
) ON s.name = rule.store_name;
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
DELETE FROM scrape_rules WHERE store_id IN (
  SELECT id FROM stores WHERE name IN ('Sigmaelectrónica', 'Electronilab', 'Vistronica')
);
DELETE FROM stores WHERE name IN ('Sigmaelectrónica', 'Electronilab', 'Vistronica');
-- +goose StatementEnd
