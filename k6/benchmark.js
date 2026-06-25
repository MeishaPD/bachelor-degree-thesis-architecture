import http from 'k6/http';
import { sleep, check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'https://localhost:8081';
const ENDPOINT = __ENV.ENDPOINT || 'ping';
const VUS = Number(__ENV.VUS || 10);
const DURATION = __ENV.DURATION || '1m';
const SLEEP = Number(__ENV.SLEEP || 0.1);

const DB_PRODUCT_COUNT = Number(__ENV.DB_PRODUCT_COUNT || 205);
const DB_ORDER_COUNT = Number(__ENV.DB_ORDER_COUNT || 2002);
const DB_CUSTOMER_COUNT = Number(__ENV.DB_CUSTOMER_COUNT || 503);

const MEMORY_PRODUCT_IDS = [101, 102, 103, 104, 105, 106, 107, 108];

export const options = {
  scenarios: {
    benchmark: {
      executor: 'constant-vus',
      vus: VUS,
      duration: DURATION,
      gracefulStop: '10s',
    }
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
  },
  summaryTrendStats: [
    'avg',
    'min',
    'med',
    'max',
    'p(90)',
    'p(95)',
    'p(99)',
  ]
};

const headers = {
  "Content-Type": "application/json",
};

function stableNumber(max) {
  return ((__VU * 997 + __ITER) % max) + 1;
}

function stableMemoryProductId(offset = 0) {
  const index = ((__VU * 997 + __ITER + offset) % MEMORY_PRODUCT_IDS.length);
  return MEMORY_PRODUCT_IDS[index];
}

function requestPing() {
  const res = http.get(`${BASE_URL}/ping`, {
    tags: { endpoint: 'ping' },
  });

  check(res, {
    "ping status 200": (r) => r.status === 200,
  });
}

function requestOrderPreview() {
  const payload = JSON.stringify({
    customer_tier: 'gold',
    items: [
      { product_id: stableMemoryProductId(0), quantity: 2 },
      { product_id: stableMemoryProductId(1), quantity: 1 },
      { product_id: stableMemoryProductId(2), quantity: 3 },
      { product_id: stableMemoryProductId(3), quantity: 1 },
      { product_id: stableMemoryProductId(0), quantity: 1 },
    ]
  });

  const res = http.post(`${BASE_URL}/orders/preview`, payload, {
    headers,
    tags: { endpoint: 'order_preview' },
  });

  check(res, {
    "order_preview status 200": (r) => r.status === 200,
  });
}

function requestPricingCalculate() {
  const payload = JSON.stringify({
    customer_tier: 'gold',
    shipping_zone: 'domestic',
    voucher_code: 'HEMAT10',
    items: [
      { product_id: stableMemoryProductId(0), quantity: 3 },
      { product_id: stableMemoryProductId(1), quantity: 2 },
      { product_id: stableMemoryProductId(2), quantity: 1 },
      { product_id: stableMemoryProductId(3), quantity: 5 },
      { product_id: stableMemoryProductId(4), quantity: 2 },
    ]
  });

  const res = http.post(`${BASE_URL}/pricing/calculate`, payload, {
    headers,
    tags: { endpoint: 'pricing_calculate' }
  });

  check(res, {
    "pricing_calculate status 200": (r) => r.status === 200,
  });
}

function requestProductDetail() {
  const productId = stableNumber(DB_PRODUCT_COUNT);

  const res = http.get(`${BASE_URL}/products/${productId}`, {
    tags: { endpoint: 'product_detail' },
  })

  check(res, {
    "product_detail status 200": (r) => r.status === 200,
  });
}

function requestOrderStatusUpdate() {
  const orderId = stableNumber(DB_ORDER_COUNT);

  const statuses = ['pending', 'paid', 'shipped', 'completed', 'cancelled'];
  const status = statuses[(__VU + __ITER) % statuses.length];

  const payload = JSON.stringify({
    order_status: status,
  });

  const res = http.put(`${BASE_URL}/orders/${orderId}/status`, payload, {
    headers,
    tags: { endpoint: 'order_status_update' },
  });

  check(res, {
    "order_status_update status 200": (r) => r.status === 200,
  })
}

function requestCustomerOrderSummary() {
  const customerId = stableNumber(DB_CUSTOMER_COUNT);

  const res = http.get(`${BASE_URL}/customers/${customerId}/order-summary`, {
    tags: { endpoint: 'customer_order_summary' },
  });

  check(res, {
    "customer_order_summary status 200": (r) => r.status === 200,
  })
}

export default function () {
  switch (ENDPOINT) {
    case 'ping':
      requestPing();
      break;
    case 'order_preview':
      requestOrderPreview();
      break;
    case 'pricing':
      requestPricingCalculate();
      break;
    case 'product':
      requestProductDetail();
      break;
    case 'order_status':
      requestOrderStatusUpdate();
      break;
    case 'summary':
      requestCustomerOrderSummary();
      break;
    default:
      throw new Error(`ENDPOINT tidak dikenal: ${ENDPOINT}`);
  }

  sleep(SLEEP);
}
