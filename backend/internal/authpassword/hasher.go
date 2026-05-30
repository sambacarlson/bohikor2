package authpassword

type Hasher interface {
	Hash(plain string) (string, error)
	Verify(hashed string, plain string) bool
}

type bcryptHasher struct{}

func NewBcryptHasher() Hasher {
	return &bcryptHasher{}
}
