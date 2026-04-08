-- +goose Up

-- +goose StatementBegin
-- Sigma uses a custom Kallyas theme — selectors differ from standard WooCommerce.
-- name:  h3.kw-details-title  (not h2.woocommerce-loop-product__title)
-- image: img.kw-prodimage-img-secondary  (has src; primary img uses data-echo lazy load)
UPDATE scrape_rules
SET
  name_selector  = 'h3.kw-details-title',
  image_selector = 'img.kw-prodimage-img-secondary'
WHERE store_id = (SELECT id FROM stores WHERE name = 'Sigmaelectrónica');
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
UPDATE scrape_rules
SET
  name_selector  = 'h2.woocommerce-loop-product__title',
  image_selector = 'img.wp-post-image'
WHERE store_id = (SELECT id FROM stores WHERE name = 'Sigmaelectrónica');
-- +goose StatementEnd
