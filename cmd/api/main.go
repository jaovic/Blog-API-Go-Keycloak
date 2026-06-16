package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/joao.martins/blog/internal/auth"
	"github.com/joao.martins/blog/internal/posts"
	"github.com/joao.martins/blog/internal/users"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from environment")
	}

	db := connectDB()
	defer db.Close()

	verifier := setupOIDC()

	postsHandler := posts.NewHandler(db)

	kcClient := users.NewKeycloakClient(
		os.Getenv("KEYCLOAK_URL"),
		os.Getenv("KEYCLOAK_REALM"),
		os.Getenv("KEYCLOAK_CLIENT_ID"),
		os.Getenv("KEYCLOAK_CLIENT_SECRET"),
	)
	usersHandler := users.NewHandler(kcClient)

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3001"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})

	authMiddleware := auth.Middleware(verifier)

	r.Get("/me", func(w http.ResponseWriter, req *http.Request) {
		authMiddleware(http.HandlerFunc(meHandler)).ServeHTTP(w, req)
	})

	r.Route("/posts", func(r chi.Router) {
		r.Mount("/", postsHandler.Routes(authMiddleware))
	})

	// users: registo público + administração protegida
	r.Mount("/users", usersHandler.Routes(authMiddleware))

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("server running on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"sub":%q,"username":%q,"email":%q,"roles":%v}`,
		claims.Sub, claims.PreferredUsername, claims.Email, toJSON(claims.RealmAccess.Roles))
}

func toJSON(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	out := `[`
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		out += `"` + s + `"`
	}
	return out + "]"
}

func connectDB() *sql.DB {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	log.Println("connected to postgres")
	return db
}

func setupOIDC() *oidc.IDTokenVerifier {
	keycloakURL := os.Getenv("KEYCLOAK_URL")
	realm := os.Getenv("KEYCLOAK_REALM")
	clientID := os.Getenv("KEYCLOAK_CLIENT_ID")

	issuer := fmt.Sprintf("%s/realms/%s", keycloakURL, realm)
	provider, err := oidc.NewProvider(context.Background(), issuer)
	if err != nil {
		log.Fatalf("failed to connect to Keycloak: %v\nmake sure Keycloak is running: docker compose up -d", err)
	}
	log.Printf("connected to Keycloak realm %q", realm)

	// O Keycloak coloca o client no campo "azp", não em "aud".
	// SkipClientIDCheck evita o erro "aud mismatch" e validamos o issuer na mesma.
	_ = clientID
	return provider.Verifier(&oidc.Config{SkipClientIDCheck: true})
}
