# Blog API — Go + Keycloak

API REST de uma plataforma de blog multi-autor construída com Go e Keycloak para aprendizagem de autenticação e autorização com OIDC.

## Stack

- **Go** — chi, go-oidc, lib/pq
- **Keycloak 24** — autenticação, roles e JWT
- **PostgreSQL 16** — persistência
- **Docker Compose** — ambiente local

## Roles

| Role | Permissões |
|---|---|
| `author` | Criar, editar e submeter os próprios posts |
| `editor` | Aprovar ou rejeitar posts em revisão |
| `admin` | Gerir utilizadores e roles |

## Endpoints

### Auth (Keycloak)
| Método | Rota | Descrição |
|---|---|---|
| POST | `/realms/blog-platform/protocol/openid-connect/token` | Obter JWT |

### Health
| Método | Rota | Acesso |
|---|---|---|
| GET | `/health` | público |

### Me
| Método | Rota | Acesso |
|---|---|---|
| GET | `/me` | autenticado |

### Users
| Método | Rota | Acesso |
|---|---|---|
| POST | `/users/register` | público |
| GET | `/users` | admin |
| GET | `/users/:id` | admin |
| PATCH | `/users/:id/roles` | admin |
| DELETE | `/users/:id` | admin |

### Posts
| Método | Rota | Acesso |
|---|---|---|
| GET | `/posts` | público |
| GET | `/posts/:id` | público |
| POST | `/posts` | author |
| PUT | `/posts/:id` | author (próprio) |
| DELETE | `/posts/:id` | author / admin |
| POST | `/posts/:id/submit` | author |
| POST | `/posts/:id/approve` | editor |
| POST | `/posts/:id/reject` | editor |

### Fluxo de publicação

```
draft → pending_review → published
           ↓
         draft (rejeitado)
```

## Como rodar

### Pré-requisitos
- Docker Desktop
- Go 1.21+

### 1. Clonar e configurar

```bash
git clone https://github.com/jaovic/Blog-API-Go-Keycloak.git
cd Blog-API-Go-Keycloak
cp .env.example .env
```

### 2. Subir os containers

```bash
make docker-up
```

> O Keycloak leva ~30 segundos para inicializar. Aguarda antes de prosseguir.

### 3. Configurar o Keycloak

Acede a [http://localhost:8080](http://localhost:8080) com `admin / admin`.

**Criar o Realm:**
- Dropdown "Keycloak" → Create realm → Nome: `blog-platform`

**Criar o Client:**
- Clients → Create client → Client ID: `blog-api`
- Ativa **Client authentication** e **Service accounts roles**
- Aba Credentials → copia o **Client secret** para o `.env`

**Criar as Roles (Realm roles):**
- `author`, `editor`, `admin`

**Permissões do Service Account:**
- Clients → blog-api → Service accounts roles → Assign role
- Filtra por `realm-management` → atribui `manage-users`, `view-users`, `query-users`

### 4. Configurar o `.env`

```env
PORT=3000

DB_HOST=localhost
DB_PORT=5432
DB_USER=blog
DB_PASSWORD=blog
DB_NAME=blog

KEYCLOAK_URL=http://localhost:8080
KEYCLOAK_REALM=blog-platform
KEYCLOAK_CLIENT_ID=blog-api
KEYCLOAK_CLIENT_SECRET=<client secret copiado>
```

### 5. Rodar a migration

```bash
make migrate
```

### 6. Iniciar a API

```bash
make run
```

A API estará disponível em `http://localhost:3000`.

## Testar com Postman

Importa o ficheiro `blog-api.postman_collection.json` no Postman.

A collection inclui:
- Requests de autenticação para os 3 perfis (author, editor, admin)
- Todos os endpoints com variáveis automáticas (`{{access_token}}`, `{{post_id}}`, `{{user_id}}`)
- Scripts que guardam automaticamente o token e IDs após cada request

## Estrutura do projeto

```
.
├── cmd/api/main.go              # entry point
├── internal/
│   ├── auth/
│   │   └── middleware.go        # validação JWT + RequireRole
│   ├── posts/
│   │   ├── handler.go           # endpoints de posts
│   │   └── model.go             # struct Post
│   └── users/
│       ├── handler.go           # endpoints de users
│       ├── keycloak_client.go   # cliente da Keycloak Admin API
│       └── model.go             # structs de users
├── migrations/
│   └── 001_create_posts.sql
├── scripts/
│   └── init-db.sql              # cria schema do Keycloak no Postgres
├── docker-compose.yml
├── Makefile
└── .env.example
```

## Makefile

```bash
make run          # inicia a API
make build        # compila o binário
make docker-up    # sobe Keycloak + PostgreSQL
make docker-down  # para os containers
make migrate      # executa as migrations
make tidy         # go mod tidy
```
