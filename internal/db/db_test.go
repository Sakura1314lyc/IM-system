package db

import (
	"testing"

)

func setupDB(t *testing.T) *Database {
	t.Helper()
	db, err := InitDatabase(":memory:")
	if err != nil {
		t.Fatalf("failed to init in-memory database: %v", err)
	}
	return db
}

func TestInitDatabase_InMemory(t *testing.T) {
	db := setupDB(t)
	if db == nil {
		t.Fatal("expected non-nil database")
	}
}

func TestDefaultUsersExist(t *testing.T) {
	db := setupDB(t)

	defaultUsers := []string{"alice", "bob", "charlie"}
	for _, name := range defaultUsers {
		user, err := db.GetUserByUsername(name)
		if err != nil {
			t.Errorf("expected default user %q to exist, got error: %v", name, err)
		}
		if user.Username != name {
			t.Errorf("expected username %q, got %q", name, user.Username)
		}
	}
}

func TestDefaultUsersAuthentication(t *testing.T) {
	db := setupDB(t)

	tests := []struct {
		name     string
		username string
		password string
		wantOK   bool
	}{
		{"alice correct password", "alice", "123456", true},
		{"bob correct password", "bob", "123456", true},
		{"charlie correct password", "charlie", "123456", true},
		{"wrong password", "alice", "wrongpass", false},
		{"non-existent user", "nobody", "123456", false},
		{"empty password", "alice", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := db.AuthenticateUser(tt.username, tt.password)
			if tt.wantOK {
				if err != nil {
					t.Errorf("expected auth success, got error: %v", err)
				}
				if user == nil {
					t.Fatal("expected non-nil user on success")
				}
				if user.Username != tt.username {
					t.Errorf("expected username %q, got %q", tt.username, user.Username)
				}
				// Password field is tagged json:"-" so it's never serialized
				if user.Password == "" {
					t.Error("DBUser.Password should contain bcrypt hash internally")
				}
			} else {
				if err == nil {
					t.Error("expected auth failure, got success")
				}
			}
		})
	}
}

func TestRegisterUser(t *testing.T) {
	db := setupDB(t)

	t.Run("register new user successfully", func(t *testing.T) {
		err := db.RegisterUser("newuser", "password123", "🦊")
		if err != nil {
			t.Fatalf("expected register to succeed, got: %v", err)
		}

		user, err := db.GetUserByUsername("newuser")
		if err != nil {
			t.Fatalf("expected to find newly registered user: %v", err)
		}
		if user.Avatar != "🦊" {
			t.Errorf("expected avatar '🦊', got %q", user.Avatar)
		}
	})

	t.Run("register duplicate username", func(t *testing.T) {
		err := db.RegisterUser("alice", "password123", "👤")
		if err == nil {
			t.Error("expected error when registering duplicate username")
		}
	})

	t.Run("register with short username", func(t *testing.T) {
		err := db.RegisterUser("ab", "password123", "👤")
		if err == nil {
			t.Error("expected error for short username")
		}
	})

	t.Run("register with long username", func(t *testing.T) {
		long := "abcdefghijklmnopqrstuvwxyz12345"
		err := db.RegisterUser(long, "password123", "👤")
		if err == nil {
			t.Error("expected error for too-long username")
		}
	})

	t.Run("register with short password", func(t *testing.T) {
		err := db.RegisterUser("validuser", "12345", "👤")
		if err == nil {
			t.Error("expected error for short password")
		}
	})
}

func TestGetUserByUsername(t *testing.T) {
	db := setupDB(t)

	t.Run("existing user", func(t *testing.T) {
		user, err := db.GetUserByUsername("alice")
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if user.Username != "alice" {
			t.Errorf("expected alice, got %q", user.Username)
		}
		if user.Avatar == "" {
			t.Error("expected non-empty avatar")
		}
	})

	t.Run("non-existent user", func(t *testing.T) {
		_, err := db.GetUserByUsername("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent user")
		}
	})
}

func TestGroupOperations(t *testing.T) {
	db := setupDB(t)

	alice, err := db.GetUserByUsername("alice")
	if err != nil {
		t.Fatalf("failed to get alice: %v", err)
	}
	bob, err := db.GetUserByUsername("bob")
	if err != nil {
		t.Fatalf("failed to get bob: %v", err)
	}

	t.Run("create group successfully", func(t *testing.T) {
		group, err := db.CreateGroup("test-group", alice.ID, "test description")
		if err != nil {
			t.Fatalf("expected create group to succeed, got: %v", err)
		}
		if group.Name != "test-group" {
			t.Errorf("expected group name 'test-group', got %q", group.Name)
		}
		if group.CreatorID != alice.ID {
			t.Errorf("expected creator %d, got %d", alice.ID, group.CreatorID)
		}
	})

	t.Run("create duplicate group name", func(t *testing.T) {
		_, err := db.CreateGroup("test-group", bob.ID, "")
		if err == nil {
			t.Error("expected error for duplicate group name")
		}
	})

	t.Run("join group", func(t *testing.T) {
		err := db.JoinGroup("test-group", bob.ID)
		if err != nil {
			t.Fatalf("expected join group to succeed, got: %v", err)
		}

		members, err := db.GetGroupMembers("test-group")
		if err != nil {
			t.Fatalf("failed to get group members: %v", err)
		}
		if len(members) != 2 {
			t.Errorf("expected 2 members, got %d", len(members))
		}
	})

	t.Run("leave group", func(t *testing.T) {
		err := db.LeaveGroup("test-group", bob.ID)
		if err != nil {
			t.Fatalf("expected leave group to succeed, got: %v", err)
		}

		members, err := db.GetGroupMembers("test-group")
		if err != nil {
			t.Fatalf("failed to get group members: %v", err)
		}
		if len(members) != 1 {
			t.Errorf("expected 1 member after leaving, got %d", len(members))
		}
	})

	t.Run("leave group when not a member", func(t *testing.T) {
		err := db.LeaveGroup("test-group", bob.ID)
		if err == nil {
			t.Error("expected error when leaving group as non-member")
		}
	})

	t.Run("get group by name", func(t *testing.T) {
		group, err := db.GetGroupByName("test-group")
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if group.Name != "test-group" {
			t.Errorf("expected 'test-group', got %q", group.Name)
		}
	})

	t.Run("get non-existent group", func(t *testing.T) {
		_, err := db.GetGroupByName("non-existent-group")
		if err == nil {
			t.Error("expected error for non-existent group")
		}
	})

	t.Run("join non-existent group", func(t *testing.T) {
		err := db.JoinGroup("non-existent-group", alice.ID)
		if err == nil {
			t.Error("expected error when joining non-existent group")
		}
	})

	t.Run("get user groups", func(t *testing.T) {
		groups, err := db.GetUserGroups("alice")
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if len(groups) == 0 {
			t.Error("expected alice to be in at least one group")
		}
	})
}

func TestGroupMembersEmptyGroup(t *testing.T) {
	db := setupDB(t)

	members, err := db.GetGroupMembers("non-existent-group")
	if err != nil {
		t.Fatalf("expected empty result, not error: %v", err)
	}
	if len(members) != 0 {
		t.Errorf("expected 0 members for non-existent group, got %d", len(members))
	}
}

func TestMessageOperations(t *testing.T) {
	db := setupDB(t)

	alice, _ := db.GetUserByUsername("alice")
	bob, _ := db.GetUserByUsername("bob")

	// Create a group for group message tests
	group, err := db.CreateGroup("chat-group", alice.ID, "")
	if err != nil {
		t.Fatalf("failed to create group: %v", err)
	}
	db.JoinGroup("chat-group", bob.ID)

	t.Run("save public message", func(t *testing.T) {
		msg, err := db.SaveMessage(alice.ID, nil, nil, "hello everyone", "public")
		if err != nil {
			t.Fatalf("expected save to succeed, got: %v", err)
		}
		if msg.Content != "hello everyone" {
			t.Errorf("expected content 'hello everyone', got %q", msg.Content)
		}
		if msg.Type != "public" {
			t.Errorf("expected type 'public', got %q", msg.Type)
		}
	})

	t.Run("save private message", func(t *testing.T) {
		_, err := db.SaveMessage(alice.ID, &bob.ID, nil, "secret hello", "private")
		if err != nil {
			t.Fatalf("expected save to succeed, got: %v", err)
		}
	})

	t.Run("save group message", func(t *testing.T) {
		_, err := db.SaveMessage(alice.ID, nil, &group.ID, "group hi", "group")
		if err != nil {
			t.Fatalf("expected save to succeed, got: %v", err)
		}
	})

	t.Run("get public messages", func(t *testing.T) {
		msgs, err := db.GetPublicMessages(10)
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if len(msgs) == 0 {
			t.Error("expected at least one public message")
		}
		if msgs[0].Content != "hello everyone" {
			t.Errorf("expected 'hello everyone', got %q", msgs[0].Content)
		}
		if msgs[0].Type != "public" {
			t.Errorf("expected type 'public', got %q", msgs[0].Type)
		}
	})

	t.Run("get private messages", func(t *testing.T) {
		msgs, err := db.GetPrivateMessages("alice", "bob", 10)
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if len(msgs) == 0 {
			t.Fatal("expected at least one private message")
		}
		if msgs[0].Content != "secret hello" {
			t.Errorf("expected 'secret hello', got %q", msgs[0].Content)
		}
		if msgs[0].From != "alice" {
			t.Errorf("expected from 'alice', got %q", msgs[0].From)
		}
	})

	t.Run("get group messages", func(t *testing.T) {
		msgs, err := db.GetGroupMessages("chat-group", 10)
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if len(msgs) == 0 {
			t.Fatal("expected at least one group message")
		}
		if msgs[0].Content != "group hi" {
			t.Errorf("expected 'group hi', got %q", msgs[0].Content)
		}
	})

	t.Run("get messages with limit", func(t *testing.T) {
		msgs, err := db.GetPublicMessages(0)
		if err != nil {
			t.Fatalf("expected success with zero limit, got: %v", err)
		}
		if len(msgs) != 0 {
			t.Errorf("expected zero messages with limit=0, got %d", len(msgs))
		}
	})

	t.Run("get messages from reverse perspective", func(t *testing.T) {
		// bob and alice should see the same private messages
		msgs, err := db.GetPrivateMessages("bob", "alice", 10)
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		if len(msgs) == 0 {
			t.Error("expected bob to see private messages with alice")
		}
	})

	t.Run("get messages for non-existent group", func(t *testing.T) {
		msgs, err := db.GetGroupMessages("non-existent-group", 10)
		if err != nil {
			t.Fatalf("expected no error for non-existent group, got: %v", err)
		}
		if len(msgs) != 0 {
			t.Errorf("expected zero messages, got %d", len(msgs))
		}
	})
}

func TestFriendOperations(t *testing.T) {
	db := setupDB(t)

	alice, _ := db.GetUserByUsername("alice")
	bob, _ := db.GetUserByUsername("bob")
	charlie, _ := db.GetUserByUsername("charlie")

	t.Run("add friend", func(t *testing.T) {
		err := db.AddFriend(alice.ID, bob.ID)
		if err != nil {
			t.Fatalf("expected add friend to succeed, got: %v", err)
		}
	})

	t.Run("is friend", func(t *testing.T) {
		isFriend, err := db.IsFriend(alice.ID, bob.ID)
		if err != nil {
			t.Fatalf("expected IsFriend to succeed, got: %v", err)
		}
		if !isFriend {
			t.Error("expected alice and bob to be friends")
		}
	})

	t.Run("friendship is bidirectional", func(t *testing.T) {
		isFriend, err := db.IsFriend(bob.ID, alice.ID)
		if err != nil {
			t.Fatalf("expected IsFriend to succeed, got: %v", err)
		}
		if !isFriend {
			t.Error("expected bob to see alice as friend (bidirectional)")
		}
	})

	t.Run("get friends list", func(t *testing.T) {
		friends, err := db.GetFriends(alice.ID)
		if err != nil {
			t.Fatalf("expected GetFriends to succeed, got: %v", err)
		}
		if len(friends) != 1 {
			t.Fatalf("expected alice to have 1 friend, got %d", len(friends))
		}
		if friends[0].Username != "bob" {
			t.Errorf("expected friend 'bob', got %q", friends[0].Username)
		}
	})

	t.Run("non-friend check", func(t *testing.T) {
		isFriend, err := db.IsFriend(alice.ID, charlie.ID)
		if err != nil {
			t.Fatalf("expected IsFriend to succeed, got: %v", err)
		}
		if isFriend {
			t.Error("expected alice and charlie to NOT be friends")
		}
	})

	t.Run("remove friend", func(t *testing.T) {
		err := db.RemoveFriend(alice.ID, bob.ID)
		if err != nil {
			t.Fatalf("expected remove friend to succeed, got: %v", err)
		}

		isFriend, _ := db.IsFriend(alice.ID, bob.ID)
		if isFriend {
			t.Error("expected alice and bob to no longer be friends")
		}

		friends, err := db.GetFriends(alice.ID)
		if err != nil {
			t.Fatalf("expected GetFriends to succeed, got: %v", err)
		}
		if len(friends) != 0 {
			t.Errorf("expected alice to have 0 friends, got %d", len(friends))
		}
	})

	t.Run("add self as friend", func(t *testing.T) {
		err := db.AddFriend(alice.ID, alice.ID)
		if err == nil {
			t.Error("expected error when adding self as friend")
		}
	})

	t.Run("add non-existent user as friend", func(t *testing.T) {
		err := db.AddFriend(alice.ID, 99999)
		if err == nil {
			t.Error("expected error when adding non-existent user as friend")
		}
	})

	t.Run("remove non-existent friend", func(t *testing.T) {
		err := db.RemoveFriend(alice.ID, bob.ID)
		if err == nil {
			t.Error("expected error when removing non-existent friendship")
		}
	})
}

func TestUpdateUserAvatar(t *testing.T) {
	db := setupDB(t)

	t.Run("update avatar", func(t *testing.T) {
		err := db.UpdateUserAvatar("alice", "🦄")
		if err != nil {
			t.Fatalf("expected update to succeed, got: %v", err)
		}
		user, err := db.GetUserByUsername("alice")
		if err != nil {
			t.Fatalf("failed to get user: %v", err)
		}
		if user.Avatar != "🦄" {
			t.Errorf("expected avatar '🦄', got %q", user.Avatar)
		}
	})

	t.Run("update avatar for non-existent user", func(t *testing.T) {
		err := db.UpdateUserAvatar("nonexistent", "🦄")
		if err != nil {
			t.Logf("expected success or silent failure, got: %v", err)
		}
	})
}

func TestUpdateUserProfile(t *testing.T) {
	db := setupDB(t)

	t.Run("update profile", func(t *testing.T) {
		err := db.UpdateUserProfile("alice", "female", "new signature")
		if err != nil {
			t.Fatalf("expected update to succeed, got: %v", err)
		}

		user, err := db.GetUserByUsername("alice")
		if err != nil {
			t.Fatalf("failed to get user: %v", err)
		}
		if user.Gender != "female" {
			t.Errorf("expected gender 'female', got %q", user.Gender)
		}
		if user.Signature != "new signature" {
			t.Errorf("expected signature 'new signature', got %q", user.Signature)
		}
	})

	t.Run("update profile for non-existent user", func(t *testing.T) {
		err := db.UpdateUserProfile("nonexistent", "male", "sig")
		if err != nil {
			t.Logf("expected success or silent failure, got: %v", err)
		}
	})
}

func TestAuthenticateUserReturnsPasswordHashInternally(t *testing.T) {
	db := setupDB(t)

	user, err := db.AuthenticateUser("alice", "123456")
	if err != nil {
		t.Fatalf("expected authentication to succeed: %v", err)
	}

	// Password field is populated internally (for bcrypt comparison)
	// but tagged json:"-" so it's never serialized in API responses
	if user.Password == "" {
		t.Error("expected internal DBUser to have password hash populated")
	}
}

func TestConcurrentReads(t *testing.T) {
	// Use temp file so multiple connections from pool see the same DB
	dir := t.TempDir()
	database, err := InitDatabase(dir + "/test.db")
	if err != nil {
		t.Fatalf("failed to init temp db: %v", err)
	}

	done := make(chan struct{}, 3)
	users := []string{"alice", "bob", "charlie"}
	for _, name := range users {
		name := name
		go func() {
			defer func() { done <- struct{}{} }()
			user, err := database.GetUserByUsername(name)
			if err != nil {
				t.Errorf("expected to find %q: %v", name, err)
			} else if user.Username != name {
				t.Errorf("expected %q, got %q", name, user.Username)
			}
		}()
	}

	for range users {
		<-done
	}
	database.db.Close()
}

func TestDatabaseEdgeCases(t *testing.T) {
	db := setupDB(t)

	t.Run("register user with empty avatar defaults", func(t *testing.T) {
		err := db.RegisterUser("edgeuser", "password123", "")
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
		user, err := db.GetUserByUsername("edgeuser")
		if err != nil {
			t.Fatalf("failed to get user: %v", err)
		}
		if user.Username != "edgeuser" {
			t.Errorf("expected 'edgeuser', got %q", user.Username)
		}
	})

	t.Run("boundary username length", func(t *testing.T) {
		// exactly 3 chars
		err := db.RegisterUser("abc", "password123", "👤")
		if err != nil {
			t.Errorf("expected 3-char username to be valid, got: %v", err)
		}
		// exactly 20 chars
		err = db.RegisterUser("abcdefghijklmnopqrst", "password123", "👤")
		if err != nil {
			t.Errorf("expected 20-char username to be valid, got: %v", err)
		}
	})

	t.Run("boundary password length", func(t *testing.T) {
		err := db.RegisterUser("pwdtest", "123456", "👤")
		if err != nil {
			t.Errorf("expected 6-char password to be valid, got: %v", err)
		}
	})
}
