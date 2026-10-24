package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pionus/pionus-go/internal/model"
)

const userColumns = `id, username, email, password_hash, screen_name, url, role,
	created_at, updated_at, last_login_at`

func scanUser(sc scanner) (*model.User, error) {
	u := &model.User{}
	var lastLogin sql.NullTime
	if err := sc.Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.ScreenName, &u.URL,
		&u.Role, &u.CreatedAt, &u.UpdatedAt, &lastLogin,
	); err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		t := lastLogin.Time
		u.LastLoginAt = &t
	}
	return u, nil
}

func (s *SQLiteStore) CreateUser(u *model.User) error {
	now := time.Now()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	if u.Role == "" {
		u.Role = model.RoleEditor
	}

	res, err := s.db.Exec(
		`INSERT INTO users
			(username, email, password_hash, screen_name, url, role, created_at, updated_at, last_login_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.Username, u.Email, u.PasswordHash, u.ScreenName, u.URL, u.Role,
		u.CreatedAt, u.UpdatedAt, u.LastLoginAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: username or email already exists", ErrConflict)
		}
		return fmt.Errorf("create user: %w", err)
	}
	u.ID, _ = res.LastInsertId()
	return nil
}

func (s *SQLiteStore) GetUser(id int64) (*model.User, error) {
	return s.getUser(`id = ?`, id)
}

func (s *SQLiteStore) GetUserByUsername(username string) (*model.User, error) {
	return s.getUser(`username = ?`, username)
}

func (s *SQLiteStore) GetUserByEmail(email string) (*model.User, error) {
	return s.getUser(`email = ?`, email)
}

func (s *SQLiteStore) getUser(where string, arg any) (*model.User, error) {
	u, err := scanUser(s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE `+where, arg))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (s *SQLiteStore) ListUsers() ([]*model.User, error) {
	rows, err := s.db.Query(`SELECT ` + userColumns + ` FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]*model.User, 0)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *SQLiteStore) UpdateUser(u *model.User) error {
	u.UpdatedAt = time.Now()
	res, err := s.db.Exec(
		`UPDATE users SET
			username = ?, email = ?, screen_name = ?, url = ?, role = ?, updated_at = ?,
			password_hash = CASE WHEN ? = '' THEN password_hash ELSE ? END
		 WHERE id = ?`,
		u.Username, u.Email, u.ScreenName, u.URL, u.Role, u.UpdatedAt,
		u.PasswordHash, u.PasswordHash, u.ID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: username or email already exists", ErrConflict)
		}
		return fmt.Errorf("update user: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) DeleteUser(id int64) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) TouchUserLogin(id int64, at time.Time) error {
	_, err := s.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, at, id)
	if err != nil {
		return fmt.Errorf("touch user login: %w", err)
	}
	return nil
}

func (s *SQLiteStore) CreateSession(sess *model.Session) error {
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = time.Now()
	}
	_, err := s.db.Exec(
		`INSERT INTO sessions (token_hash, user_id, ip, user_agent, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sess.TokenHash, sess.UserID, sess.IP, sess.UserAgent, sess.CreatedAt, sess.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetSession(tokenHash string, now time.Time) (*model.Session, *model.User, error) {
	sess := &model.Session{}
	err := s.db.QueryRow(
		`SELECT token_hash, user_id, ip, user_agent, created_at, expires_at
		 FROM sessions WHERE token_hash = ? AND expires_at > ?`,
		tokenHash, now,
	).Scan(&sess.TokenHash, &sess.UserID, &sess.IP, &sess.UserAgent, &sess.CreatedAt, &sess.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("get session: %w", err)
	}

	user, err := s.GetUser(sess.UserID)
	if err != nil {
		return nil, nil, err
	}
	return sess, user, nil
}

func (s *SQLiteStore) DeleteSession(tokenHash string) error {
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteUserSessions(userID int64) error {
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteExpiredSessions(now time.Time) error {
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}
