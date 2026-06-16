#!/usr/bin/env bash
# =============================================================================
# seed.sh — Popula a base de dados com posts de exemplo
#
# Uso:
#   ./scripts/seed.sh
#
# Variáveis opcionais (lidas do .env se não definidas):
#   SEED_EMAIL     — email de um utilizador com roles author + editor/admin
#   SEED_PASSWORD  — password desse utilizador
#   API_URL        — URL da API (default: http://localhost:3000)
# =============================================================================

set -euo pipefail

# ── Configuração ──────────────────────────────────────────────────────────────
load_env() {
  if [ -f ".env" ]; then
    export $(grep -v '^#' .env | grep -v '^$' | xargs)
  fi
}
load_env

API_URL="${API_URL:-http://localhost:3000}"
KC_URL="${KEYCLOAK_URL:-http://localhost:8080}"
KC_REALM="${KEYCLOAK_REALM:-blog-platform}"
KC_CLIENT_ID="${KEYCLOAK_CLIENT_ID:-blog-api}"
KC_CLIENT_SECRET="${KEYCLOAK_CLIENT_SECRET:-}"
SEED_EMAIL="${SEED_EMAIL:-}"
SEED_PASSWORD="${SEED_PASSWORD:-}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
ok()   { echo -e "${GREEN}✓${NC} $*"; }
warn() { echo -e "${YELLOW}⚠${NC}  $*"; }
fail() { echo -e "${RED}✗${NC} $*"; exit 1; }

# ── Verificações ──────────────────────────────────────────────────────────────
command -v curl  >/dev/null || fail "curl não encontrado"
command -v jq    >/dev/null || fail "jq não encontrado — instala com: brew install jq"

curl -sf "$API_URL/health" > /dev/null || fail "API não está acessível em $API_URL — faz 'make run' primeiro"

if [ -z "$SEED_EMAIL" ] || [ -z "$SEED_PASSWORD" ]; then
  echo ""
  echo "Credenciais do utilizador seed (precisa de ter roles author + editor ou admin):"
  read -rp "  Email: "    SEED_EMAIL
  read -rsp "  Password: " SEED_PASSWORD
  echo ""
fi

# ── Autenticação ──────────────────────────────────────────────────────────────
echo ""
echo "A autenticar em Keycloak..."

TOKEN=$(curl -sf -X POST "$KC_URL/realms/$KC_REALM/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password&client_id=$KC_CLIENT_ID&client_secret=$KC_CLIENT_SECRET&username=$SEED_EMAIL&password=$SEED_PASSWORD&scope=openid" \
  | jq -r '.access_token') || fail "Falha ao obter token — verifica as credenciais"

[ "$TOKEN" = "null" ] || [ -z "$TOKEN" ] && fail "Token inválido — verifica as credenciais e as roles do utilizador"
ok "Token obtido"

# ── Helpers ───────────────────────────────────────────────────────────────────
AUTH="-H \"Authorization: Bearer $TOKEN\""

post_create() {
  curl -sf -X POST "$API_URL/posts" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "$1" | jq -r '.id'
}

post_submit() {
  curl -sf -X POST "$API_URL/posts/$1/submit" \
    -H "Authorization: Bearer $TOKEN" > /dev/null
}

post_approve() {
  curl -sf -X POST "$API_URL/posts/$1/approve" \
    -H "Authorization: Bearer $TOKEN" > /dev/null
}

# ── Limpeza (opcional) ────────────────────────────────────────────────────────
echo ""
read -rp "Apagar posts existentes antes de criar os exemplos? [s/N] " CLEAN
if [[ "$CLEAN" =~ ^[sS]$ ]]; then
  EXISTING=$(curl -sf "$API_URL/posts" | jq '.[].id' 2>/dev/null || echo "")
  if [ -n "$EXISTING" ]; then
    for id in $EXISTING; do
      curl -sf -X DELETE "$API_URL/posts/$id" \
        -H "Authorization: Bearer $TOKEN" > /dev/null 2>&1 || true
    done
    ok "Posts existentes removidos"
  else
    warn "Sem posts para remover"
  fi
fi

# ── Seed ──────────────────────────────────────────────────────────────────────
echo ""
echo "A criar posts..."

# ── Publicados ────────────────────────────────────────────────────────────────
ID=$(post_create '{
  "title": "Como o Go mudou a minha forma de pensar sobre APIs",
  "content": "Vim do Node.js e Express depois de 4 anos. A transição para Go não foi fácil — tipagem estrita, sem exceptions, sem NPM com 400 mil pacotes. Mas três meses depois, não volto atrás.\n\nO que mais me surpreendeu foi a simplicidade do modelo de concorrência. Em Node.js estava habituado a callbacks, depois promises, depois async/await. Em Go existe um modelo de goroutines e channels que, uma vez entendido, parece óbvio.\n\nOutra diferença brutal: o tempo de compilação e o binário final. O meu projeto atual compila em 2 segundos e o binário tem 12MB — sem runtime externo, sem node_modules com 200MB.\n\nAinda sinto falta do ecossistema do Node.js em algumas áreas, especialmente para prototipagem rápida. Mas para APIs de produção com requisitos de performance? Go ganhou a minha confiança."
}')
post_submit "$ID"; post_approve "$ID"
ok "Publicado: \"Como o Go mudou a minha forma de pensar sobre APIs\""

ID=$(post_create '{
  "title": "Keycloak em produção: o que ninguém te conta",
  "content": "Quando comecei a integrar o Keycloak fiquei impressionado com a documentação. Depois fiquei menos impressionado quando cheguei à prática.\n\nO primeiro problema foi o campo `aud` no JWT. O Keycloak coloca o client no campo `azp`, não em `aud`. Qualquer biblioteca OIDC standard vai rejeitar o token por padrão. Solução: `SkipClientIDCheck: true`.\n\nO segundo problema foi a service account para o Admin REST API. Criar o utilizador via Keycloak Admin API exige que a service account tenha as roles `manage-users`, `view-users` e `query-users` do realm `master`. Não do realm da aplicação — do master. Isto não está documentado de forma clara.\n\nO terceiro problema foram os redirect URIs. Em desenvolvimento é tentador colocar `*` mas em produção tens de ser preciso. E o Keycloak é muito literal.\n\nDito isto, o Keycloak é uma solução enterprise sólida. SSO, MFA, federação de identidades, social login — tudo configurável sem uma linha de código. Vale o investimento de aprendizagem."
}')
post_submit "$ID"; post_approve "$ID"
ok "Publicado: \"Keycloak em produção: o que ninguém te conta\""

ID=$(post_create '{
  "title": "PostgreSQL vs MongoDB: a escolha que define o projeto",
  "content": "Trabalho com bases de dados há 6 anos e já cometi o erro de escolher a ferramenta errada para o trabalho.\n\nPostgreSQL é a escolha certa quando:\n- Os teus dados têm relações claras (utilizadores, posts, comentários, roles)\n- Precisas de transações ACID reais\n- A consistência é mais importante que a velocidade de escrita\n\nMongoDB faz sentido quando:\n- Os teus documentos são verdadeiramente independentes entre si\n- O schema muda frequentemente durante o desenvolvimento\n- Tens dados hierárquicos complexos que seriam dolorosos em tabelas relacionais\n\nO erro mais comum: usar MongoDB porque parece mais fácil no início. É — até ao dia em que precisas de fazer um join entre duas coleções. Nesse dia, vais querer ter usado PostgreSQL.\n\nA minha recomendação: começa sempre com PostgreSQL. Se tiveres um caso de uso específico que o MongoDB resolve melhor, migra essa parte. Nunca ao contrário."
}')
post_submit "$ID"; post_approve "$ID"
ok "Publicado: \"PostgreSQL vs MongoDB: a escolha que define o projeto\""

ID=$(post_create '{
  "title": "Docker Compose para quem vem do desenvolvimento local clássico",
  "content": "Durante anos desenvolvi com XAMPP e instalações locais de MySQL e Redis. Depois alguém me mostrou o Docker Compose e percebi que tinha estado a perder tempo.\n\nO conceito central é simples: defines os serviços que a tua aplicação precisa num ficheiro YAML e o Docker trata do resto. Precisas de PostgreSQL 16? `docker compose up` e tens. Keycloak? `docker compose up`.\n\nO que mudou no meu workflow:\n\n1. Onboarding de novos devs: antes levava 2 dias a configurar o ambiente. Agora é `git clone` + `docker compose up` + `make migrate`.\n2. Ambiente consistente: acabaram os \"na minha máquina funciona\".\n3. Múltiplos projetos em paralelo: posso ter o PostgreSQL do projeto A na porta 5432 e o do projeto B na 5433, sem conflitos.\n\nO único cuidado: volumes de dados. Se fizeres `docker compose down -v` perdes tudo."
}')
post_submit "$ID"; post_approve "$ID"
ok "Publicado: \"Docker Compose para quem vem do desenvolvimento local clássico\""

ID=$(post_create '{
  "title": "Next.js App Router: o que mudou do Pages Router e porquê importa",
  "content": "Migrei um projeto de Pages Router para App Router e foi uma das decisões técnicas mais impactantes do último ano.\n\nA mudança fundamental é o modelo de rendering. No Pages Router, cada página era um componente React client-side com opções de SSR/SSG. No App Router, tudo é Server Component por defeito. Só marcas como `\"use client\"` quando realmente precisas de interatividade no browser.\n\nIsso tem implicações enormes:\n\nPerformance: componentes que apenas exibem dados não enviam JavaScript para o cliente.\n\nData fetching: em vez de `getServerSideProps`, fazes `async/await` diretamente no componente.\n\nLayouts nested: podes ter layouts partilhados por grupos de rotas, com loading states independentes por segmento.\n\nA curva de aprendizagem é real — especialmente entender onde colocar `\"use client\"`. Mas o resultado final é mais simples e mais performático."
}')
post_submit "$ID"; post_approve "$ID"
ok "Publicado: \"Next.js App Router: o que mudou do Pages Router e porquê importa\""

# ── Em revisão ────────────────────────────────────────────────────────────────
ID=$(post_create '{
  "title": "TailwindCSS vs CSS Modules: dois anos de experiência real",
  "content": "Usei CSS Modules durante dois anos em projetos React. Depois mudei para Tailwind e nunca mais voltei — mas entendo quem prefere Modules.\n\nO argumento principal contra o Tailwind é sempre o mesmo: \"o HTML fica ilegível com 30 classes\". É verdade para quem não está habituado. Depois de umas semanas, passa a ser natural.\n\nO que o Tailwind faz bem:\n- Zero ficheiros CSS para gerir\n- Consistência de design system automática\n- Purging automático — só o CSS usado vai para produção\n- Developer experience brutal com autocomplete no VSCode\n\nO que CSS Modules faz melhor:\n- Componentes verdadeiramente isolados\n- Sem conflito de naming em projetos grandes\n\nA minha recomendação atual: Tailwind para projetos novos, CSS Modules para projetos enterprise com equipas grandes."
}')
post_submit "$ID"
ok "Em revisão: \"TailwindCSS vs CSS Modules: dois anos de experiência real\""

ID=$(post_create '{
  "title": "JWT vs Sessions: qual usar em 2025",
  "content": "Esta questão aparece em toda a entrevista técnica e a resposta correta é sempre: depende.\n\nJWT faz sentido quando:\n- Tens múltiplos serviços que precisam de validar a identidade (microserviços)\n- O teu cliente não é um browser (app mobile, CLI)\n- Queres autenticação stateless\n\nSessions fazem sentido quando:\n- Estás a construir uma aplicação web tradicional com browser\n- Precisas de invalidar sessões instantaneamente\n- A segurança de roubo de token é uma preocupação real\n\nO problema que ninguém menciona com JWT: a revogação. Um JWT válido por 1 hora não podes invalidar sem uma blacklist — o que destrói o argumento do stateless.\n\nA minha stack atual usa Keycloak com tokens de curta duração (5 minutos) e refresh tokens com rotação. É o melhor dos dois mundos."
}')
post_submit "$ID"
ok "Em revisão: \"JWT vs Sessions: qual usar em 2025\""

ID=$(post_create '{
  "title": "Monorepo com Turborepo: vale a complexidade?",
  "content": "A nossa equipa migrou para monorepo há 6 meses. Este é o balanço honesto.\n\nO problema que tentávamos resolver: tínhamos 4 repositórios separados (API, frontend, mobile, shared-types) e a sincronização de tipos entre eles era um pesadelo.\n\nCom o monorepo e Turborepo:\n✅ Tipos partilhados com import real, não cópia-cola\n✅ Um único git blame para toda a feature\n✅ CI mais inteligente — só testa o que mudou\n\n❌ Setup inicial demorou 3 dias\n❌ Tempos de git clone e git status aumentaram\n❌ A curva de aprendizagem para novos devs é maior\n\nO veredito: para equipas de 3+ pessoas com projetos interligados, compensa. Para um projeto solo ou com repos verdadeiramente independentes, é over-engineering."
}')
post_submit "$ID"
ok "Em revisão: \"Monorepo com Turborepo: vale a complexidade?\""

# ── Rascunhos ─────────────────────────────────────────────────────────────────
ID=$(post_create '{
  "title": "Microserviços em Go: quando faz sentido e quando é overkill",
  "content": "Tenho visto muitas startups a adotar microserviços porque \"é o que as grandes empresas fazem\". Este artigo é um aviso.\n\nA Netflix, Amazon e Google usam microserviços porque têm centenas de equipas a trabalhar em paralelo. Se és uma startup de 3 pessoas, isso não se aplica.\n\nO custo de microserviços que ninguém conta:\n- Latência de rede entre serviços\n- Distributed tracing — precisas de ferramentas como Jaeger ou Zipkin\n- Gestão de transações distribuídas — esquece ACID, bem-vindo ao mundo dos sagas\n- Infraestrutura: Kubernetes, service mesh, load balancers por serviço\n\nA minha recomendação: começa com um monolito bem estruturado. Quando tiveres um bottleneck real de escala, extrais esse módulo para um serviço. Não antes."
}')
ok "Rascunho: \"Microserviços em Go: quando faz sentido e quando é overkill\""

ID=$(post_create '{
  "title": "Observabilidade em produção: logs, métricas e traces",
  "content": "Coloquei a primeira aplicação Go em produção sem observabilidade adequada. Aprendi da pior forma.\n\nUm utilizador reportou que a aplicação estava lenta. Sem métricas, não sabia se era a base de dados, a rede ou o código. Sem traces, não conseguia perceber onde o tempo estava a ser gasto.\n\nHoje o meu setup mínimo para qualquer projeto em produção:\n\nLogs estruturados com slog (built-in desde Go 1.21): cada log tem campos JSON — request_id, user_id, duration_ms.\n\nMétricas com Prometheus: exponho um endpoint /metrics com contadores de requests e histogramas de latência.\n\nTraces com OpenTelemetry: cada request tem um trace ID que o acompanha por todos os serviços.\n\n(rascunho — falta a parte de alertas)"
}')
ok "Rascunho: \"Observabilidade em produção: logs, métricas e traces\""

ID=$(post_create '{
  "title": "Git Flow vs Trunk-Based Development: a minha experiência",
  "content": "Usei Git Flow durante 3 anos. Depois mudei para Trunk-Based Development. Não tenho certeza qual prefiro — depende muito da equipa.\n\nGit Flow faz sentido quando:\n- Tens releases formais com versioning (v1.2.3)\n- A QA precisa de tempo para testar antes de cada release\n- Tens múltiplas versões em produção simultaneamente\n\nTrunk-Based é melhor quando:\n- Deploys contínuos para produção (várias vezes por dia)\n- Equipa com boa cobertura de testes automatizados\n- Feature flags para esconder funcionalidades incompletas\n\nNeste projeto estou a usar uma variante simples: branch main protegida + branch develop para features + PRs obrigatórios.\n\n(rascunho — preciso de adicionar exemplos concretos)"
}')
ok "Rascunho: \"Git Flow vs Trunk-Based Development: a minha experiência\""

# ── Resumo ────────────────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " Seed concluído"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo " ✅  5 posts publicados"
echo " 🟡  3 posts em revisão"
echo " 📝  3 rascunhos"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
