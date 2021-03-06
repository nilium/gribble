package main

import (
	"io"
	"sync"

	"github.com/tv42/zbase32"
	"golang.org/x/crypto/bcrypt"
)

func genToken(length int, rand io.Reader) (string, error) {
	key := make([]byte, length)
	if _, err := io.ReadFull(rand, key); err != nil {
		return "", err
	}
	token := zbase32.EncodeToString(key)
	return token, nil
}

type randomToken struct {
	length int
	rand   io.Reader

	m     sync.RWMutex
	token string
	hash  []byte
}

func newRandomToken(length int, rand io.Reader) (*randomToken, error) {
	tok := &randomToken{
		length: length,
		rand:   rand,
	}
	if err := tok.Refresh(); err != nil {
		return nil, err
	}
	return tok, nil
}

func (r *randomToken) Refresh() error {
	const hashStrength = 11

	r.m.Lock()
	defer r.m.Unlock()

	token, err := genToken(r.length, r.rand)
	hash, err := bcrypt.GenerateFromPassword([]byte(token), hashStrength)
	if err != nil {
		return err
	}

	r.token, r.hash = token, hash
	return nil
}

func (r *randomToken) Token() (string, []byte) {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.token, r.hash
}

func (r *randomToken) Compare(token string) error {
	_, hash := r.Token()
	return bcrypt.CompareHashAndPassword(hash, []byte(token))
}
