package server

import (
	"context"
	"errors"
	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"strings"
)

type AdminIdentity struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	CreatedAt    int64  `json:"created_at"`
	LastSignInAt *int64 `json:"last_sign_in_at"`
	Banned       bool   `json:"banned"`
	Locked       bool   `json:"locked"`
}

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

func ClerkIdentity(ctx context.Context, id string) (Identity, error) {
	u, err := user.Get(ctx, id)
	if err != nil {
		return Identity{}, err
	}
	return verifiedIdentity(u)
}
func DeleteClerkIdentity(ctx context.Context, id string) error {
	_, err := user.Delete(ctx, id)
	var apiErr *clerk.APIErrorResponse
	if errors.As(err, &apiErr) && apiErr.HTTPStatusCode == 404 {
		return nil
	}
	return err
}

func ListClerkIdentities(ctx context.Context) ([]AdminIdentity, error) {
	pageSize := int64(500)
	var out []AdminIdentity
	for offset := int64(0); ; offset += pageSize {
		page, err := user.List(ctx, &user.ListParams{ListParams: clerk.ListParams{Limit: &pageSize, Offset: &offset}})
		if err != nil {
			return nil, err
		}
		for _, u := range page.Users {
			if u == nil {
				continue
			}
			email := ""
			if u.PrimaryEmailAddressID != nil {
				for _, candidate := range u.EmailAddresses {
					if candidate != nil && candidate.ID == *u.PrimaryEmailAddressID {
						email = strings.ToLower(candidate.EmailAddress)
						break
					}
				}
			}
			if email == "" && len(u.EmailAddresses) > 0 && u.EmailAddresses[0] != nil {
				email = strings.ToLower(u.EmailAddresses[0].EmailAddress)
			}
			name := strings.TrimSpace(strings.TrimSpace(pointerValue(u.FirstName)) + " " + strings.TrimSpace(pointerValue(u.LastName)))
			if name == "" {
				name = email
			}
			out = append(out, AdminIdentity{ID: u.ID, Email: email, Name: name, CreatedAt: u.CreatedAt, LastSignInAt: u.LastSignInAt, Banned: u.Banned, Locked: u.Locked})
		}
		if int64(len(page.Users)) < pageSize || int64(len(out)) >= page.TotalCount {
			break
		}
	}
	return out, nil
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
