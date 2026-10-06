# 🎬 GINFLIX APP

A simple streaming platform proof of concept for educational purposes.

This software is a proof of concept of a very simple streaming platform used for education purposes (in particular introduction to K8s, DevSecOps). 
It is distributed under GPL v3 license. 

## 📝 Foreword

**Author:** Adrien Maes (maes@telecom-paris.fr)

This software is a proof of concept of a very simple streaming platform used for education purposes. It is distributed under **GPL v3 license**.

---

## 📋 Prerequisites

- Go (latest stable version)
- Docker & Docker Compose

---

## 🚀 Quick Start

### 1. Configure Environment Variables

Create a `.env` file in the root directory:

```bash
cp .env.example .env
```

Then fill in the required values:

| Variable | Description | Example |
|----------|-------------|---------|
| `MONGO_URI` | MongoDB connection string | `mongodb://localhost:27017/ginflix` |
| `GARAGE_ENDPOINT` | S3-compatible storage endpoint | `https://storage.example.com` |
| `GARAGE_BUCKET` | Bucket name for video storage | `videos` |
| `GARAGE_ACCESS_KEY` | Access key for S3 storage | - |
| `GARAGE_SECRET_KEY` | Secret key for S3 storage | - |
| `GARAGE_USE_SSL` | Use HTTPS for S3 connection | `true` or `false` |
| `BACKEND_URL` | Backend service URL | `http://localhost:8080` |
| `STREAM_URL` | Stream service URL | `http://localhost:8081` |
| `KEYCLOAK_CLIENT_ID` | Keycloak client ID (if auth enabled) | - |
| `KEYCLOAK_HOST` | Keycloak server host | `https://auth.example.com` |
| `KEYCLOAK_REALM` | Keycloak realm | - |
| `ENV` | Environment (production/development) | `production` |

### 2. Configure Frontend Services

Edit the `ARGS` parameter in these Dockerfiles to point to your API endpoints:
- `frontend/Dockerfile`
- `frontend-admin/Dockerfile`

### 3. Install Dependencies

```bash
cd backend && go mod tidy
cd ../streamer && go mod tidy
cd ..
```

### 4. Start Services

```bash
docker compose up -d --build
```

---

## 📚 Documentation

### API Documentation

- **Backend & Streamer Endpoints:** Available in OpenAPI format
- **Swagger UI:** Available at `http://localhost:8000`

### Authentication

This application uses Keycloak for authentication. For details on the authentication flow:
- [Auth0 Authorization Code Flow with PKCE](https://auth0.com/docs/get-started/authentication-and-authorization-flow/authorization-code-flow-with-pkce)

**Note:** Authentication can be disabled by setting `AUTH_DISABLED=true` in your `.env` file.

---

## 📦 Project Structure

```
├── backend/              # Backend API service (Go)
├── streamer/            # Stream service (Go)
├── frontend/            # User frontend (React/Next.js)
├── frontend-admin/      # Admin frontend
├── docker-compose.yml   # Container orchestration
├── .env.example         # Environment variables template
└── README.md            # This file
```

---

## 🔧 Environment Setup

For detailed environment variable configuration, see `.env.example` in the root directory.

**Security Note:** In production, sensitive credentials should be stored in Kubernetes Secrets or a secure secrets management system, not in ConfigMaps.

---

## 📄 License

This program is free software: you can redistribute it and/or modify it under the terms of the **GNU General Public License v3** as published by the Free Software Foundation.

This program is distributed in the hope that it will be useful, but **WITHOUT ANY WARRANTY**; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

See the [GNU General Public License v3](https://www.gnu.org/licenses/) for more details.

