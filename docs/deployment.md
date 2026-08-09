# Deployment

Tidak ada Docker sama sekali di stack ini — Go dikompilasi menjadi satu
binary statis, jadi server tidak butuh apa pun selain binary tersebut,
PostgreSQL, dan Caddy.

## Susunan di server

```
/opt/app/
  api               # binary HTTP server
  cli               # binary CLI (migrasi, provisioning, cleanup)
  api.backup        # binary sebelumnya, untuk rollback instan

/etc/app/api.env    # konfigurasi, chmod 600
/etc/systemd/system/app-api.service
/etc/systemd/system/app-cleanup.service
/etc/systemd/system/app-cleanup.timer
```

## Setup pertama kali

```bash
sudo useradd -r -s /usr/sbin/nologin app
sudo mkdir -p /opt/app /etc/app
sudo chown app:app /opt/app
git clone <your-repo-url> /opt/app
sudo cp deploy/app-api.service deploy/app-cleanup.service deploy/app-cleanup.timer /etc/systemd/system/
sudo cp .env.example /etc/app/api.env   # lalu edit dengan nilai sebenarnya
sudo chmod 600 /etc/app/api.env
sudo systemctl daemon-reload
sudo systemctl enable --now app-cleanup.timer
```

Pasang Caddy, lalu:
```bash
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile   # edit domainnya lebih dulu
sudo systemctl reload caddy
```

### Ubuntu/Debian vs Rocky

Yang berbeda hanya package manager-nya (`apt install postgresql caddy` vs
`dnf install postgresql-server caddy`); selebihnya — unit systemd, binary
Go, script deploy — identik.

**Khusus Rocky:** SELinux aktif secara bawaan dan memblokir Caddy untuk
terhubung ke `localhost:8080` kecuali diizinkan:
```bash
sudo setsebool -P httpd_can_network_connect 1
```
Tanpa ini, request yang lewat Caddy gagal dengan pesan "Permission denied"
yang sama sekali tidak terlihat berhubungan dengan SELinux — kalau Caddy
tidak bisa menjangkau API di Rocky, ini hal pertama yang harus dicek.

## Melakukan deploy pembaruan

```bash
cd /opt/app
./deploy/deploy.sh
```
Script ini melakukan pull, membackup binary yang sedang berjalan,
build ulang, menjalankan migrasi platform + tenant, menyinkronkan permission,
lalu me-restart service. Perkirakan downtime 1–2 detik saat restart —
jadwalkan deploy di luar jam operasional untuk aplikasi dengan jam kerja
tetap (seperti POS).

### Rollback

```bash
cp api.backup api && sudo systemctl restart app-api
```

## Backup

```bash
sudo cp deploy/pg-backup.service deploy/pg-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now pg-backup.timer
```
Ini menjalankan `pg_dump` setiap hari dan menyimpan 35 hari terakhir secara
lokal. **Salin backup ke luar server** (rsync/rclone ke penyimpanan remote) —
backup yang berada di disk yang sama dengan disk yang rusak tidak melindungi
apa pun. Uji proses restore secara berkala; backup yang tidak pernah diuji
itu tebakan, bukan backup.

## Environment variable

Lihat `.env.example` untuk daftar lengkapnya. Semuanya bisa dibaca/diubah
tanpa build ulang — edit `/etc/app/api.env` lalu jalankan
`systemctl restart app-api`.
