package db_types

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type (
	User struct {
		ID             primitive.ObjectID `bson:"_id"`
		Username       string             `bson:"username"`
		HashedPassword string             `bson:"hashed_password"`
		TelegramID     int                `bson:"telegram_id,omitempty"`
	}

	LoginData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
)
