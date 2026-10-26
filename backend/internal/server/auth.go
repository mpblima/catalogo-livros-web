package server

import (
	"context"
	"errors"
	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"strings"
)

type Identity struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}
type Authenticator func(context.Context, string) (Identity, error)

func ClerkAuth(origins []string) Authenticator {
	return func(ctx context.Context, token string) (Identity, error) {
		claims, err := jwt.Verify(ctx, &jwt.VerifyParams{Token: token, AuthorizedPartyHandler: func(azp string) bool {
			for _, origin := range origins {
				if azp == origin {
					return true
				}
			}
			return false
		}})
		if err != nil {
			return Identity{}, err
		}
		u, err := user.Get(ctx, claims.Subject)
		if err != nil {
			return Identity{}, err
		}
		return verifiedIdentity(u)
	}
}
func verifiedIdentity(u *clerk.User) (Identity, error) {
	if u.Banned || u.Locked {
		return Identity{}, errors.New("account unavailable")
	}
	if u.PrimaryEmailAddressID != nil {
		for _, email := range u.EmailAddresses {
			if email.ID == *u.PrimaryEmailAddressID && email.Verification != nil && email.Verification.Status == "verified" {
				name := ""
				if u.FirstName != nil {
					name = *u.FirstName
				}
				if u.LastName != nil {
					name += " " + *u.LastName
				}
				if strings.TrimSpace(name) == "" {
					name = email.EmailAddress
				}
				return Identity{u.ID, strings.ToLower(email.EmailAddress), strings.TrimSpace(name)}, nil
			}
		}
	}
	return Identity{}, errors.New("verified primary email required")
}
