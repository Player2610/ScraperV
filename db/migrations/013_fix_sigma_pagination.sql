-- +goose Up

-- +goose StatementBegin
-- Sigma uses Kallyas theme pagination, not WooCommerce default.
-- Correct next-page selector: a.pagination-item-next-link
UPDATE scrape_rules
SET pagination_selector = 'a.pagination-item-next-link'
WHERE store_id = (SELECT id FROM stores WHERE name = 'Sigmaelectrónica');
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
UPDATE scrape_rules
SET pagination_selector = 'a.next.page-numbers'
WHERE store_id = (SELECT id FROM stores WHERE name = 'Sigmaelectrónica');
-- +goose StatementEnd
