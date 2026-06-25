# Thesis Go Architecture Benchmark

Repository ini berisi implementasi backend berbasis Go untuk eksperimen perbandingan **Layered Architecture** dan **Hexagonal Architecture**. Proyek ini digunakan sebagai studi kasus dalam penelitian skripsi mengenai trade-off antara performa runtime, penggunaan sumber daya, kompleksitas struktur, dan modifiability pada backend berbasis Go.

## Tujuan Repository

Repository ini dibuat untuk menyediakan dua implementasi backend dengan fungsi bisnis yang setara, tetapi menggunakan pendekatan arsitektur yang berbeda:

1. **Layered Architecture**
2. **Hexagonal Architecture**

Kedua implementasi dirancang agar memiliki endpoint, logika bisnis, struktur request/response, dan data uji yang setara. Dengan demikian, hasil pengujian dapat lebih merepresentasikan dampak keputusan arsitektural, bukan perbedaan fitur atau logika aplikasi.

## Studi Kasus

Studi kasus yang digunakan adalah backend e-commerce sederhana. Domain ini dipilih karena memiliki pola operasi umum pada sistem backend, seperti:

* request ringan;
* validasi input;
* perhitungan harga;
* operasi baca data;
* operasi tulis data;
* agregasi data sederhana.

Pada tahap awal, repository ini menyediakan endpoint yang berfokus pada request ringan dan proses application-bound tanpa akses basis data.

## Struktur Repository

```text
.
├── layered-backend/
│   ├── cmd/
│   │   └── api/
│   └── internal/
│       ├── handler/
│       ├── service/
│       ├── repository/
│       ├── model/
│       └── dto/
│
├── hexagonal-backend/
│   ├── cmd/
│   │   └── api/
│   └── internal/
│       ├── adapter/
│       │   ├── inbound/
│       │   └── outbound/
│       └── core/
│           ├── domain/
│           ├── port/
│           └── usecase/
│
└── README.md
```

## Layered Architecture

Implementasi Layered Architecture disusun menggunakan alur umum:

```text
router → handler → service → repository → data source
```

Pada pendekatan ini:

* `handler` menerima request HTTP dan membentuk response;
* `service` menjalankan validasi, orkestrasi proses, dan logika bisnis;
* `repository` menangani akses data atau data referensi;
* `model` berisi struktur data internal;
* `dto` digunakan untuk request dan response HTTP.

Layered Architecture digunakan sebagai baseline pembanding karena memiliki struktur yang relatif sederhana dan umum digunakan pada backend berbasis API.

## Hexagonal Architecture

Implementasi Hexagonal Architecture disusun menggunakan pendekatan ports and adapters.

Alur umumnya adalah:

```text
router → inbound adapter → input port → use case/domain → output port → outbound adapter
```

Pada pendekatan ini:

* `adapter/inbound` menerima request dari luar aplikasi, seperti HTTP request;
* `adapter/inbound/http/dto` berisi DTO untuk request dan response HTTP;
* `core/domain` berisi model dan aturan bisnis inti;
* `core/usecase` mengorkestrasi alur aplikasi;
* `core/port` berisi kontrak input dan output;
* `adapter/outbound` berisi implementasi teknis untuk kebutuhan eksternal atau data source.

Hexagonal Architecture digunakan untuk mengamati dampak boundary tambahan seperti port, adapter, use case, dan domain terhadap performa runtime, kompleksitas struktur, serta modifiability.

## Endpoint

Endpoint awal yang tersedia:

| Method | Endpoint             | Deskripsi                                                                        |
| ------ | -------------------- | -------------------------------------------------------------------------------- |
| `GET`  | `/ping`              | Endpoint ringan untuk health check dan baseline request                          |
| `POST` | `/orders/preview`    | Simulasi ringkasan pesanan tanpa penyimpanan data                                |
| `POST` | `/pricing/calculate` | Perhitungan harga berdasarkan item, tier pelanggan, zona pengiriman, dan voucher |

## Menjalankan Layered Backend

```bash
cd layered-backend
go run ./cmd/api
```

Default port:

```text
8081
```

Contoh akses:

```bash
curl http://localhost:8081/ping
```

## Menjalankan Hexagonal Backend

```bash
cd hexagonal-backend
go run ./cmd/api
```

Default port:

```text
8082
```

Contoh akses:

```bash
curl http://localhost:8082/ping
```

## Contoh Request Pricing

Layered Backend:

```bash
curl -X POST http://localhost:8081/pricing/calculate \
  -H "Content-Type: application/json" \
  -d '{"customer_tier":"gold","shipping_zone":"domestic","voucher_code":"HEMAT10","items":[{"product_id":101,"quantity":2},{"product_id":103,"quantity":1}]}'
```

Hexagonal Backend:

```bash
curl -X POST http://localhost:8082/pricing/calculate \
  -H "Content-Type: application/json" \
  -d '{"customer_tier":"gold","shipping_zone":"domestic","voucher_code":"HEMAT10","items":[{"product_id":101,"quantity":2},{"product_id":103,"quantity":1}]}'
```

## Fokus Pengujian

Repository ini digunakan untuk mendukung beberapa jenis pengukuran dalam penelitian, yaitu:

1. **Performa runtime**

   * response time;
   * throughput.

2. **Penggunaan sumber daya**

   * CPU utilization;
   * memory usage;
   * RSS;
   * VMS.

3. **Kompleksitas struktur**

   * production LOC;
   * jumlah source file;
   * jumlah package;
   * jumlah interface/port;
   * dependency internal.

4. **Modifiability**

   * changed files;
   * changed packages;
   * affected architectural components;
   * code churn;
   * relative churn;
   * contract/interface changes.

## Catatan Pengembangan

Kedua backend dalam repository ini dibuat dengan prinsip kesetaraan fungsi. Perbedaan utama antara kedua implementasi terletak pada struktur arsitektur dan arah dependensi antarkomponen. Oleh karena itu, perubahan fitur, endpoint, atau logika bisnis sebaiknya diterapkan secara setara pada kedua implementasi agar validitas eksperimen tetap terjaga.

## Status

Repository ini masih dikembangkan untuk kebutuhan eksperimen skripsi. Endpoint, skenario pengujian, dan struktur kode dapat berubah mengikuti kebutuhan penelitian.