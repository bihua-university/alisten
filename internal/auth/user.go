package auth

type User struct {
	Name string `json:"name"`
	// Email salted md5 of email
	Email string `json:"email"`
}
