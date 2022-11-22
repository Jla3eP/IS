package api

import (
	"IS/blockchain/config"
	"IS/blockchain/database_utils/hashing"
	dbtypes "IS/blockchain/database_utils/types"
	"IS/utils"
	"context"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

var (
	usersCollection *mongo.Collection
	stateCollection *mongo.Collection
	ctx             = context.Background()
	databaseInited  = false
)

const idAndNicknameToSaltFormat = "%s+%s" //id.InsertedID.(primitive.ObjectID).String(), user.Username

func UserExists(ctx context.Context, userName string) bool {
	filter := bson.M{"nickname": userName}
	resp := usersCollection.FindOne(ctx, filter)
	if resp.Err() != nil {
		return false
	}
	return true
}

func GetBlocks() Blocks {
	if !databaseInited {
		panic("use database_utils.InitDB()")
	}
	opts := options.Find().SetSort(bson.D{{"number", 1}})
	blocks := make(Blocks, 0)
	cursor, err := stateCollection.Find(ctx, bson.D{}, opts)
	if err != nil {
		return nil
	}
	err = cursor.All(context.TODO(), &blocks)
	if err != nil {
		return nil
	}

	return blocks
}

func CreateUser(ctx context.Context, user dbtypes.User, clearPassword string) error {
	if !databaseInited {
		panic("use database_utils.InitDB()")
	}
	if UserExists(ctx, user.Username) {
		return errors.New(fmt.Sprintf("user with nickname=\"%s\" exists", user.Username))
	}

	userData := bson.M{
		"nickname":   user.Username,
		"created_at": time.Now(),
	}

	id, err := usersCollection.InsertOne(ctx, userData)
	if err != nil {
		return err
	}

	filter := bson.D{{"nickname", user.Username}}
	user.ID = id.InsertedID.(primitive.ObjectID)
	salt := infoToSalt(user)

	userData["hashed_password"] = hash.CreateSaltPasswordHash(salt, clearPassword)
	update := bson.D{
		{"$set", userData},
	}

	_, err = usersCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func VerifyPassword(ctx context.Context, user dbtypes.User, clearPassword string) (bool, error) {
	if !databaseInited {
		panic("use database_utils.InitDB()")
	}
	filter := bson.D{{"nickname", user.Username}}

	if len(user.HashedPassword) == 0 {
		err := usersCollection.FindOne(ctx, filter).Decode(&user)
		if err != nil {
			return false, err
		}
	}

	salt := infoToSalt(user)
	hashPassword := hash.CreateSaltPasswordHash(salt, clearPassword)

	if !utils.AreEqual(hashPassword, user.HashedPassword) {
		return false, errors.New("invalid password")
	}

	return true, nil
}

func GetIdByUsername(username string) (primitive.ObjectID, error) {
	if !databaseInited {
		panic("use database_utils.InitDB()")
	}
	user := dbtypes.User{}
	filter := bson.D{{"nickname", username}}

	if err := usersCollection.FindOne(ctx, filter).Decode(&user); err != nil {
		return [12]byte{}, err
	}

	return user.ID, nil
}

func SetTelegramID(username string, ID int) error {
	if !databaseInited {
		panic("use database_utils.InitDB()")
	}
	filter := bson.D{{"nickname", username}}

	user := dbtypes.User{}
	err := usersCollection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		return errors.New(fmt.Sprintf("user with nickname=\"%s\" doesn't exists", username))
	}

	opts := options.Update().SetUpsert(true)
	update := bson.D{{"$set", bson.D{{"telegram_id", ID}}}}
	_, err = usersCollection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}

	return nil
}

func BlocksExist() bool {
	if !databaseInited {
		panic("use database_utils.InitDB()")
	}

	block, _ := GetLastBlock()
	return block != nil
}

func GetLastBlock() (*Block, error) {
	if !databaseInited {
		panic("use database_utils.InitDB()")
	}
	opts := options.FindOne().SetSort(bson.D{{"number", 1}})
	LFB := stateCollection.FindOne(ctx, bson.D{}, opts)
	block := &Block{}
	err := LFB.Decode(block)
	if err != nil {
		return nil, fmt.Errorf("no blocks")
	}
	return block, nil
}

func GetBlockByHash(hash Hash) (*Block, error) {
	if !databaseInited {
		panic("use database_utils.InitDB()")
	}
	opts := options.FindOne().SetSort(bson.D{{"number", 1}})
	filter := bson.D{{"hash", hash}}
	block := &Block{}
	err := stateCollection.FindOne(ctx, filter, opts).Decode(block)
	if err != nil {
		return nil, fmt.Errorf("cant find block by hash=%v", hash)
	}
	return block, nil
}

func GetBlockByNumber(number utils.BlockNumber) (*Block, error) {
	if !databaseInited {
		panic("use database_utils.InitDB()")
	}
	opts := options.FindOne().SetSort(bson.D{{"number", 1}})
	filter := bson.D{{"number", number}}
	block := &Block{}
	err := stateCollection.FindOne(ctx, filter, opts).Decode(block)
	if err != nil {
		return nil, fmt.Errorf("cant find block by number=%v", number)
	}
	return block, nil
}

func WriteBlock(block *Block) error {
	if !databaseInited {
		panic("use database_utils.InitDB()")
	}
	blockBSON, err := bson.Marshal(block)
	if err != nil {
		return err
	}
	_, err = stateCollection.InsertOne(ctx, blockBSON)
	if err != nil {
		return err
	}
	return nil
}

func infoToSalt(usr dbtypes.User) string {
	return fmt.Sprintf(idAndNicknameToSaltFormat, usr.ID.String(), usr.Username)
}

func InitDB(cfg *config.Config) {
	dataBaseAddress := cfg.DataBaseAddress
	dataBasePort := cfg.DataBasePort

	clientOptions := options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%s/", dataBaseAddress, dataBasePort))
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	usersCollection = client.Database(cfg.DataBaseName).Collection(cfg.UsersCollectionName)
	stateCollection = client.Database(cfg.DataBaseName).Collection(cfg.StateCollectionName)

	databaseInited = true
}

func StateBlock() Block {
	block := Block{
		Number:       0,
		Transactions: nil,
		ParentHash:   [HashLen]byte{},
	}
	block.GetHash()
	return block
}
