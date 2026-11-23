package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/McConfig/telemt-admin/internal/auth"
	"github.com/McConfig/telemt-admin/internal/config"
	"github.com/McConfig/telemt-admin/internal/store"
	"golang.org/x/term"
)

// Run выполняет CLI-команду.
func Run(ctx context.Context, args []string, cfg *config.Config, stdin io.Reader, stdout io.Writer) error {
	if len(args) < 2 || args[0] != "user" {
		return errors.New("использование: tgadmin user add|passwd|list|disable|enable")
	}
	st, err := store.Open(ctx, cfg.ValkeyURL, cfg.ValkeyPassword)
	if err != nil {
		return err
	}
	defer st.Close()

	switch args[1] {
	case "add":
		name, role, err := parseAdd(args[2:])
		if err != nil {
			return err
		}
		password, err := readPasswordTwice(stdin, stdout)
		if err != nil {
			return err
		}
		hash, err := auth.Hash(password)
		if err != nil {
			return err
		}
		if err := st.CreateUser(ctx, store.User{
			Name: name, Hash: hash, Role: role, CreatedAt: time.Now().UTC(),
		}); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "пользователь %s создан\n", name)
		return nil
	case "passwd":
		if len(args) != 3 {
			return errors.New("использование: tgadmin user passwd <name>")
		}
		password, err := readPasswordTwice(stdin, stdout)
		if err != nil {
			return err
		}
		hash, err := auth.Hash(password)
		if err != nil {
			return err
		}
		if err := st.SetPassword(ctx, args[2], hash); err != nil {
			return err
		}
		_ = st.DeleteUserSessions(ctx, args[2], "")
		fmt.Fprintf(stdout, "пароль %s изменён\n", args[2])
		return nil
	case "list":
		users, err := st.ListUsers(ctx)
		if err != nil {
			return err
		}
		for _, u := range users {
			state := "включён"
			if u.Disabled {
				state = "отключён"
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", u.Name, u.Role, state)
		}
		return nil
	case "disable", "enable":
		if len(args) != 3 {
			return fmt.Errorf("использование: tgadmin user %s <name>", args[1])
		}
		disabled := args[1] == "disable"
		if err := st.SetDisabled(ctx, args[2], disabled); err != nil {
			return err
		}
		if disabled {
			return st.DeleteUserSessions(ctx, args[2], "")
		}
		return nil
	default:
		return fmt.Errorf("неизвестная команда %q", args[1])
	}
}

func parseAdd(args []string) (string, store.Role, error) {
	role := store.RoleAdmin
	name := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--role":
			if i+1 >= len(args) {
				return "", "", errors.New("--role требует значение")
			}
			role = store.Role(args[i+1])
			i++
		default:
			name = args[i]
		}
	}
	if name == "" {
		return "", "", errors.New("не указано имя")
	}
	if role != store.RoleAdmin && role != store.RoleViewer {
		return "", "", fmt.Errorf("роль %q: допустимы admin, viewer", role)
	}
	return name, role, nil
}

func readPasswordTwice(in io.Reader, out io.Writer) (string, error) {
	first, err := readSecret(in, out, "Пароль: ")
	if err != nil {
		return "", err
	}
	second, err := readSecret(in, out, "Повторите пароль: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("пароли не совпадают")
	}
	if err := auth.CheckPolicy(first); err != nil {
		return "", err
	}
	return first, nil
}

func readSecret(in io.Reader, out io.Writer, prompt string) (string, error) {
	fmt.Fprint(out, prompt)
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		raw, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(out)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
	return readLine(in)
}

func readLine(in io.Reader) (string, error) {
	buf := make([]byte, 0, 128)
	one := make([]byte, 1)
	for {
		n, err := in.Read(one)
		if n == 0 || err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if one[0] == '\n' {
			break
		}
		if one[0] != '\r' {
			buf = append(buf, one[0])
		}
	}
	return string(buf), nil
}
