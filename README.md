# 🏃‍♂️ Ejecutar Backend de Finanzas Personales (Go)

Este proyecto es el backend de una app móvil de finanzas personales desarrollado en Go con el framework Gin y PostgreSQL.

## 📦 Requisitos

- Go 1.20 o superior
- PostgreSQL (se recomienda usar [Neon](https://neon.tech) para producción)
- Git

## ⚙️ Variables de Entorno

Crea un archivo `.env` en la raíz del proyecto con el siguiente contenido:

```env
PORT=8000
POSTGRES_URL=postgres://usuario:contraseña@tu-host.neon.tech:5432/tu_basededatos
SECRET_WORD=clave_super_secreta
ENV=develop
```
## Pasos de instalacion

1. Clonar el repositorio
```bash
git clone git@github.com:Befo0/pdm-backend.git
cd pdm-backend
```

2. Instalar dependencias
```bash
go mod tidy
```

3. Ejecutar la migracion de la base de datos
```bash
go run cmd/migrations/main.go
```

4. Levantar el servidor
```bash
go run main.go
```

Estara escuchandose en el puerto http://localhost:8000
