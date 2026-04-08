-- +goose Up

-- +goose StatementBegin
-- Fix Sigmaelectrónica: the old URL path /categoria/componentes-electronicos/ returns 404.
-- Correct WooCommerce path is /categoria-producto/componentes-discretos/page/{page}/
UPDATE scrape_rules
SET catalog_url_pattern = 'https://www.sigmaelectronica.net/categoria-producto/componentes-discretos/page/{page}/'
WHERE store_id = (SELECT id FROM stores WHERE name = 'Sigmaelectrónica');
-- +goose StatementEnd

-- +goose StatementBegin
-- Disable Electronilab: site migrated to Algolia InstantSearch (JS-rendered).
-- The goquery HTML scraper cannot extract products. Requires a dedicated API client.
UPDATE stores SET is_active = false WHERE name = 'Electronilab';
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
UPDATE scrape_rules
SET catalog_url_pattern = 'https://www.sigmaelectronica.net/categoria/componentes-electronicos/?page={page}'
WHERE store_id = (SELECT id FROM stores WHERE name = 'Sigmaelectrónica');
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE stores SET is_active = true WHERE name = 'Electronilab';
-- +goose StatementEnd
