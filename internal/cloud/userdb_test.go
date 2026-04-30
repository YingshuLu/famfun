package cloud

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestCreateUser(t *testing.T) {
	store := newTestStore(t)

	user, err := store.CreateUser("alice", "password123", "guest")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("got username %q, want %q", user.Username, "alice")
	}
	if user.ID == "" {
		t.Error("expected non-empty user ID")
	}
}

func TestCreateUserDuplicateUsername(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.CreateUser("alice", "pass1", "guest"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := store.CreateUser("alice", "pass2", "guest"); err == nil {
		t.Fatal("expected error for duplicate username")
	}
}

func TestCreateUserPasswordIsHashed(t *testing.T) {
	store := newTestStore(t)

	user, err := store.CreateUser("alice", "password123", "guest")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.PasswordHash == "password123" {
		t.Error("password hash should not equal plaintext")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("password123")); err != nil {
		t.Errorf("bcrypt verify failed: %v", err)
	}
}

func TestGetUserByUsername(t *testing.T) {
	store := newTestStore(t)

	created, _ := store.CreateUser("alice", "pass", "guest")
	found, err := store.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("got ID %q, want %q", found.ID, created.ID)
	}
}

func TestGetUserByUsernameNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.GetUserByUsername("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestGetUserByID(t *testing.T) {
	store := newTestStore(t)

	created, _ := store.CreateUser("alice", "pass", "guest")
	found, err := store.GetUserByID(created.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if found.Username != "alice" {
		t.Errorf("got username %q, want %q", found.Username, "alice")
	}
}

func TestGetUserByIDNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.GetUserByID("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent user ID")
	}
}

func TestCreateUserRole(t *testing.T) {
	store := newTestStore(t)

	admin, err := store.CreateUser("admin1", "pass", "admin")
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}
	if admin.Role != "admin" {
		t.Errorf("got role %q, want %q", admin.Role, "admin")
	}

	guest, err := store.CreateUser("guest1", "pass", "guest")
	if err != nil {
		t.Fatalf("CreateUser guest: %v", err)
	}
	if guest.Role != "guest" {
		t.Errorf("got role %q, want %q", guest.Role, "guest")
	}

	defaultRole, err := store.CreateUser("other1", "pass", "unknown")
	if err != nil {
		t.Fatalf("CreateUser unknown role: %v", err)
	}
	if defaultRole.Role != "guest" {
		t.Errorf("got role %q, want %q", defaultRole.Role, "guest")
	}
}

func TestGetUserByUsernameIncludesRole(t *testing.T) {
	store := newTestStore(t)

	store.CreateUser("admin2", "pass", "admin")
	found, err := store.GetUserByUsername("admin2")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if found.Role != "admin" {
		t.Errorf("got role %q, want %q", found.Role, "admin")
	}
}

func TestListUsers(t *testing.T) {
	store := newTestStore(t)

	store.CreateUser("alice", "pass", "admin")
	store.CreateUser("bob", "pass", "member")
	store.CreateUser("charlie", "pass", "guest")

	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	// +1 for seeded demo user
	if len(users) != 4 {
		t.Fatalf("got %d users, want 4", len(users))
	}
}

func TestDeleteUser(t *testing.T) {
	store := newTestStore(t)

	user, _ := store.CreateUser("alice", "pass", "guest")
	if err := store.DeleteUser(user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	_, err := store.GetUserByID(user.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestDeleteUserNotFound(t *testing.T) {
	store := newTestStore(t)

	if err := store.DeleteUser("nonexistent-id"); err != nil {
		t.Fatalf("DeleteUser nonexistent should not error: %v", err)
	}
}

func TestUpdateUserRole(t *testing.T) {
	store := newTestStore(t)

	user, _ := store.CreateUser("alice", "pass", "guest")
	if err := store.UpdateUserRole(user.ID, "admin"); err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}

	updated, _ := store.GetUserByID(user.ID)
	if updated.Role != "admin" {
		t.Errorf("got role %q, want %q", updated.Role, "admin")
	}
}

func TestUpdateUserRoleInvalid(t *testing.T) {
	store := newTestStore(t)

	user, _ := store.CreateUser("alice", "pass", "guest")
	if err := store.UpdateUserRole(user.ID, "superuser"); err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestUpdateUserPassword(t *testing.T) {
	store := newTestStore(t)

	user, _ := store.CreateUser("alice", "oldpass", "guest")
	if err := store.UpdateUserPassword(user.ID, "newpass"); err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}

	updated, _ := store.GetUserByUsername("alice")
	if err := bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("newpass")); err != nil {
		t.Errorf("new password verify failed: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("oldpass")); err == nil {
		t.Error("old password should no longer work")
	}
}
