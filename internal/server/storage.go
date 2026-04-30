package server

import "IM-system/internal/model"

// Storage defines the database operations required by the server.
type Storage interface {
	AuthenticateUser(username, password string) (*model.DBUser, error)
	RegisterUser(username, password, avatar string) error
	GetUserByUsername(username string) (*model.UserPublic, error)
	UpdateUserAvatar(username, avatar string) error
	UpdateUserProfile(username, gender, signature string) error

	SaveMessage(fromID int, toID *int, groupID *int, content, msgType string) (*model.DBMessage, error)
	GetPublicMessages(limit int) ([]*model.DBMessageExt, error)
	GetGroupMessages(groupName string, limit int) ([]*model.DBMessageExt, error)
	GetPrivateMessages(username, peer string, limit int) ([]*model.DBMessageExt, error)

	CreateGroup(name string, creatorID int, description string) (*model.DBGroup, error)
	JoinGroup(groupName string, userID int) error
	LeaveGroup(groupName string, userID int) error
	GetGroupMembers(groupName string) ([]*model.UserPublic, error)
	GetGroupByName(name string) (*model.DBGroup, error)
	GetUserGroups(username string) ([]string, error)

	AddFriend(userID, friendID int) error
	RemoveFriend(userID, friendID int) error
	GetFriends(userID int) ([]*model.UserPublic, error)
	IsFriend(userID, friendID int) (bool, error)
}
