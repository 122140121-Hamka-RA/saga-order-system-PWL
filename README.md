# Sistem Create Order Saga

Repositori ini berisi implementasi pola Saga untuk mengelola transaksi terdistribusi di beberapa microservices. Sistem ini terdiri dari tiga microservices (Order, Payment, dan Shipping) serta Saga Orchestrator yang mengkoordinasikan alur transaksi dan menangani tindakan kompensasi jika terjadi kegagalan.

## Struktur Proyek

- `order-service/`: Implementasi layanan Order
- `payment-service/`: Implementasi layanan Pembayaran
- `shipping-service/`: Implementasi layanan Pengiriman
- `orchestrator/`: Implementasi Saga Orchestrator
- `test-scenarios.go`: Skenario pengujian untuk kasus sukses dan gagal
- `documentation.md`: Dokumentasi rinci tentang sistem

## Layanan

### Order Service (Port 8081)
- `POST /create-order`: Membuat pesanan baru dengan status PENDING
- `POST /cancel-order`: Membatalkan pesanan yang ada (tindakan kompensasi)
- `GET /order-status`: Mengembalikan status pesanan

### Payment Service (Port 8082)
- `POST /process-payment`: Memproses pembayaran untuk pesanan
- `POST /refund-payment`: Mengembalikan pembayaran (tindakan kompensasi)
- `GET /payment-status`: Mengembalikan status pembayaran

### Shipping Service (Port 8083)
- `POST /start-shipping`: Memulai pengiriman untuk pesanan
- `POST /cancel-shipping`: Membatalkan pengiriman (tindakan kompensasi)
- `GET /shipping-status`: Mengembalikan status pengiriman

### Saga Orchestrator (Port 8080)
- `POST /create-order-saga`: Memulai Saga Pembuatan Pesanan
- `GET /transaction-status`: Mengembalikan status transaksi saga

## Running the System

   Untuk menjalankan sistem ikuti langkah ini:

   1. Mulai the Order Service:
      ```
      cd order-service
      go run main.go
      ```

   2. Mulai the Payment Service:
      ```
      cd payment-service
      go run main.go
      ```

   3. Mulai the Shipping Service:
      ```
      cd shipping-service
      go run main.go
      ```

   4. Mulai the Saga Orchestrator:
      ```
      cd orchestrator
      go run main.go
      ```

   5. Jalankan the test scenarios:
      ```
      go run test-scenarios.go
      ```

## Implementasi Pola Saga

Sistem ini mengimplementasikan pola Saga dengan pendekatan **Orchestration**, di mana seorang koordinator pusat (_orchestrator_) mengarahkan layanan peserta dan mengelola alur transaksi.

### Alur Transaksi
1. **Membuat Pesanan**: Orchestrator memanggil Order Service untuk membuat pesanan baru dengan status PENDING.
2. **Memproses Pembayaran**: Jika pembuatan pesanan berhasil, orchestrator memanggil Payment Service untuk memproses pembayaran.
3. **Memulai Pengiriman**: Jika pemrosesan pembayaran berhasil, orchestrator memanggil Shipping Service untuk memulai pengiriman.
4. **Menyelesaikan Transaksi**: Jika semua langkah berhasil, transaksi ditandai sebagai COMPLETED.

### Tindakan Kompensasi
Jika ada langkah yang gagal dalam transaksi, orchestrator akan menjalankan tindakan kompensasi untuk membatalkan perubahan yang sudah dilakukan oleh langkah-langkah sebelumnya:

- **Jika Pengiriman gagal**:
  - Batalkan pengiriman (jika perlu)
  - Kembalikan pembayaran
  - Batalkan pesanan

- **Jika Pembayaran gagal**:
  - Batalkan pesanan
