package users

// User representa um utilizador retornado pela Keycloak Admin API.
type User struct {
	ID        string   `json:"id"`
	Username  string   `json:"username"`
	Email     string   `json:"email"`
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Enabled   bool     `json:"enabled"`
	Roles     []string `json:"roles,omitempty"`
}

// RegisterInput é o body esperado no POST /users/register.
type RegisterInput struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Password  string `json:"password"`
}

func (i *RegisterInput) Validate() string {
	if i.Username == "" {
		return "username is required"
	}
	if i.Email == "" {
		return "email is required"
	}
	if i.Password == "" {
		return "password is required"
	}
	return ""
}

// AssignRolesInput é o body esperado no PATCH /users/:id/roles.
type AssignRolesInput struct {
	Roles []string `json:"roles"`
}
