package main

import (
	"context"
	"errors"
	"regexp"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoAlbumStore struct {
	collection *mongo.Collection
}

func NewMongoAlbumStore(collection *mongo.Collection) MongoAlbumStore {
	return MongoAlbumStore{collection: collection}
}

func (s MongoAlbumStore) List(ctx context.Context, query string, skip, limit int) ([]Album, int64, error) {
	filter := bson.M{}
	if query != "" {
		search := primitive.Regex{Pattern: regexp.QuoteMeta(query), Options: "i"}
		filter["$or"] = bson.A{
			bson.M{"title": search},
			bson.M{"artist": search},
		}
	}

	total, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	findOptions := options.Find().
		SetSort(bson.D{{Key: "title", Value: 1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))
	cursor, err := s.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	albums := make([]Album, 0)
	if err := cursor.All(ctx, &albums); err != nil {
		return nil, 0, err
	}
	return albums, total, nil
}

func (s MongoAlbumStore) Get(ctx context.Context, id string) (Album, error) {
	var album Album
	err := s.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&album)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Album{}, ErrNotFound
	}
	return album, err
}

func (s MongoAlbumStore) Create(ctx context.Context, album Album) (Album, error) {
	album.ID = primitive.NewObjectID().Hex()
	_, err := s.collection.InsertOne(ctx, album)
	return album, err
}

func (s MongoAlbumStore) Update(ctx context.Context, id string, album Album) (Album, error) {
	update := bson.M{"$set": bson.M{
		"title":      album.Title,
		"artist":     album.Artist,
		"price":      album.Price,
		"updated_at": album.UpdatedAt,
	}}

	var updated Album
	err := s.collection.FindOneAndUpdate(
		ctx,
		bson.M{"_id": id},
		update,
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Album{}, ErrNotFound
	}
	return updated, err
}

func (s MongoAlbumStore) Delete(ctx context.Context, id string) error {
	result, err := s.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}
