package configx

import "github.com/McConfig/telemt-admin/internal/auth"

func hashPassword(p string) (string, error) { return auth.Hash(p) }
