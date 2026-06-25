SET search_path TO thesis_ecommerce;

-- =========================================================
-- 1. Tambah kategori benchmark jika belum ada
-- =========================================================

INSERT INTO categories (category_name, slug) VALUES
('Peripheral', 'peripheral'),
('Accessory', 'accessory'),
('Display', 'display'),
('Camera', 'camera'),
('Audio', 'audio'),
('Storage', 'storage')
ON CONFLICT (slug) DO NOTHING;

-- =========================================================
-- 2. Tambah 500 customer benchmark
-- =========================================================

INSERT INTO customers (full_name, email, phone)
SELECT
    'Benchmark Customer ' || g,
    'bench_customer_' || g || '@example.com',
    '0899' || LPAD(g::text, 8, '0')
FROM generate_series(1, 500) AS g
ON CONFLICT (email) DO NOTHING;

-- =========================================================
-- 3. Tambah 200 product benchmark
-- =========================================================

INSERT INTO products (category_id, sku, product_name, price, stock_qty)
SELECT
    c.category_id,
    'BENCH-SKU-' || LPAD(g::text, 5, '0'),
    'Benchmark Product ' || g,
    (50000 + ((g % 100) * 10000))::numeric(12,2),
    1000 + (g % 200)
FROM generate_series(1, 200) AS g
JOIN LATERAL (
    SELECT category_id
    FROM categories
    WHERE slug = (
        ARRAY[
            'electronics',
            'peripheral',
            'accessory',
            'display',
            'camera',
            'audio',
            'storage'
        ]
    )[((g - 1) % 7) + 1]
) c ON TRUE
ON CONFLICT (sku) DO NOTHING;

-- =========================================================
-- 4. Tambah 2000 order benchmark
-- =========================================================

WITH benchmark_customers AS (
    SELECT
        customer_id,
        ROW_NUMBER() OVER (ORDER BY customer_id) AS rn
    FROM customers
    WHERE email LIKE 'bench_customer_%@example.com'
),
customer_count AS (
    SELECT COUNT(*) AS total
    FROM benchmark_customers
)
INSERT INTO orders (
    customer_id,
    order_number,
    order_status,
    order_date,
    total_amount
)
SELECT
    bc.customer_id,
    'BENCH-ORD-' || LPAD(g::text, 7, '0'),
    (
        ARRAY[
            'pending',
            'paid',
            'shipped',
            'completed',
            'cancelled'
        ]
    )[((g - 1) % 5) + 1],
    NOW() - ((g % 60) || ' days')::interval,
    0
FROM generate_series(1, 2000) AS g
JOIN customer_count cc ON cc.total > 0
JOIN benchmark_customers bc
    ON bc.rn = ((g - 1) % cc.total) + 1
ON CONFLICT (order_number) DO NOTHING;

-- =========================================================
-- 5. Tambah 3 order item untuk setiap benchmark order
-- =========================================================

WITH benchmark_orders AS (
    SELECT
        order_id,
        ROW_NUMBER() OVER (ORDER BY order_id) AS rn
    FROM orders
    WHERE order_number LIKE 'BENCH-ORD-%'
),
benchmark_products AS (
    SELECT
        product_id,
        price,
        ROW_NUMBER() OVER (ORDER BY product_id) AS rn
    FROM products
    WHERE sku LIKE 'BENCH-SKU-%'
),
product_count AS (
    SELECT COUNT(*) AS total
    FROM benchmark_products
),
generated_items AS (
    SELECT
        bo.order_id,
        bp.product_id,
        ((bo.rn + item_offset.n) % 5) + 1 AS quantity,
        bp.price AS unit_price
    FROM benchmark_orders bo
    CROSS JOIN generate_series(0, 2) AS item_offset(n)
    JOIN product_count pc ON pc.total > 0
    JOIN benchmark_products bp
        ON bp.rn = ((bo.rn + item_offset.n - 1) % pc.total) + 1
)
INSERT INTO order_items (
    order_id,
    product_id,
    quantity,
    unit_price
)
SELECT
    order_id,
    product_id,
    quantity,
    unit_price
FROM generated_items
ON CONFLICT (order_id, product_id) DO NOTHING;

-- =========================================================
-- 6. Hitung ulang total order benchmark
-- =========================================================

UPDATE orders o
SET total_amount = x.total_amount
FROM (
    SELECT
        order_id,
        SUM(subtotal) AS total_amount
    FROM order_items
    GROUP BY order_id
) x
WHERE o.order_id = x.order_id
  AND o.order_number LIKE 'BENCH-ORD-%';

-- =========================================================
-- 7. Refresh statistik PostgreSQL
-- =========================================================

ANALYZE thesis_ecommerce.categories;
ANALYZE thesis_ecommerce.customers;
ANALYZE thesis_ecommerce.products;
ANALYZE thesis_ecommerce.orders;
ANALYZE thesis_ecommerce.order_items;

-- =========================================================
-- 8. Cek jumlah data
-- =========================================================

SELECT 'categories' AS table_name, COUNT(*) AS total FROM categories
UNION ALL
SELECT 'customers', COUNT(*) FROM customers
UNION ALL
SELECT 'products', COUNT(*) FROM products
UNION ALL
SELECT 'orders', COUNT(*) FROM orders
UNION ALL
SELECT 'order_items', COUNT(*) FROM order_items;