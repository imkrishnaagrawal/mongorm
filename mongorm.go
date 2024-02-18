package mongorm

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type OrmModel struct {
	ID          *primitive.ObjectID `gorm:"primaryKey;autoIncrement" json:"id,omitempty" bson:"_id,omitempty"`
	DateCreated *time.Time          `gorm:"index;not null;default:current_timestamp" json:"date_created,omitempty" bson:"date_created,omitempty"`
	DateUpdated *time.Time          `gorm:"index;not null;default:current_timestamp" json:"date_updated,omitempty" bson:"date_updated,omitempty"`
	DateDeleted *time.Time          `gorm:"index" json:"date_deleted,omitempty" bson:"date_deleted,omitempty"`
}

func (d *OrmModel) BeforeCreate() {
	now := time.Now()
	d.DateCreated = &now
	d.DateUpdated = &now
}

func (d *OrmModel) BeforeSave() {
	now := time.Now()
	d.DateUpdated = &now
}

func (d *OrmModel) BeforeDelete() {
	now := time.Now()
	d.DateDeleted = &now
}

type MongoORM struct {
	client             *mongo.Client
	database           string
	filter             interface{}
	Error              error
	RowsAffected       uint
	UpdateResult       *mongo.UpdateResult
	PreloadCollections []string
	session            mongo.Session
	inSession          bool
	collection         *mongo.Collection
	ctx                context.Context
	fields             bson.M
}

func (orm *MongoORM) Begin() *MongoORM {
	if orm.client == nil {
		// Handle error: client not initialized
		return orm
	}

	var err error
	orm.session, err = orm.client.StartSession()
	if err != nil {
		// Handle error
		return orm
	}
	orm.inSession = true
	orm.session.StartTransaction()
	return orm
}

// Rollback aborts the current transaction and ends the session.
func (orm *MongoORM) Rollback() *MongoORM {
	if orm.inSession && orm.session != nil {
		if err := orm.session.AbortTransaction(context.Background()); err != nil {
			orm.Error = err
		}
		orm.session.EndSession(context.Background())
		orm.inSession = false
	}
	return orm
}

// Commit commits the current transaction and ends the session.
func (orm *MongoORM) Commit() *MongoORM {
	if orm.inSession && orm.session != nil {
		if err := orm.session.CommitTransaction(context.Background()); err != nil {
			orm.Error = err
		}
		orm.session.EndSession(context.Background())
		orm.inSession = false
	}
	return orm
}

func NewMongoORM(client *mongo.Client, database string) *MongoORM {
	return &MongoORM{client: client, database: database}
}

func (orm *MongoORM) Where(query string, args ...interface{}) *MongoORM {

	if query == "id = ?" && len(args) > 0 {
		// Convert the first argument to string assuming it's the ID
		idStr, ok := args[0].(string)
		if !ok {
			orm.Error = fmt.Errorf("id argument must be a string")
			return orm
		}

		// Convert string ID to primitive.ObjectID
		id, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			orm.Error = err
			return orm
		}

		orm.filter = bson.M{"_id": id}
	} else {
		// For other queries, implement as needed
	}

	return orm
}

func (orm *MongoORM) determineCollectionName(doc interface{}) string {
	t := reflect.TypeOf(doc)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice {
		// If it's a slice, get the element type of the slice
		t = t.Elem()
		if t.Kind() == reflect.Ptr {
			// If the slice element is a pointer, get the type it points to
			t = t.Elem()
		}
	}

	return fmt.Sprintf("%ss", strings.ToLower(t.Name()))
}

func (orm *MongoORM) First(doc interface{}, id ...string) *MongoORM {

	if len(id) > 0 && id[0] != "" {
		objectId, err := primitive.ObjectIDFromHex(id[0])
		if err != nil {
			orm.Error = err
			return orm
		}
		orm.filter = bson.M{"_id": objectId}
	}

	collectionName := orm.determineCollectionName(doc)

	collection := orm.client.Database(orm.database).Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := collection.FindOne(ctx, orm.filter).Decode(doc)
	orm.filter = nil
	orm.Error = err
	orm.processPreloads(doc)
	return orm
}

func (orm *MongoORM) Find(docs interface{}, filters ...interface{}) *MongoORM {

	if len(filters) > 0 {
		orm.filter, _ = filters[0].(bson.M)
	} else {
		if orm.filter != nil {
			orm.filter = orm.filter.(bson.M)
		}
	}

	collectionName := orm.determineCollectionName(docs)

	collection := orm.client.Database(orm.database).Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.M{})

	if err != nil {

		orm.Error = err
		return orm
	}

	if err := cursor.All(ctx, docs); err != nil {
		orm.Error = err
		return orm
	}
	resultVal := reflect.ValueOf(docs)
	if resultVal.Elem().Len() == 0 {
		sliceType := resultVal.Elem().Type()
		newSlice := reflect.MakeSlice(sliceType, 0, 0)
		resultVal.Elem().Set(newSlice)
	}

	orm.filter = nil
	orm.Error = err

	docsValue := reflect.ValueOf(docs).Elem()

	if docsValue.Kind() == reflect.Slice {
		for i := 0; i < docsValue.Len(); i++ {
			doc := docsValue.Index(i)
			docPtr := doc.Addr().Interface()
			orm.processPreloads(docPtr)
		}
	}

	return orm
}

func (orm *MongoORM) Create(doc interface{}) *MongoORM {
	collectionName := orm.determineCollectionName(doc)
	collection := orm.client.Database(orm.database).Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	if beforeCreater, ok := doc.(interface{ BeforeCreate() }); ok {
		beforeCreater.BeforeCreate()
	}

	result, err := collection.InsertOne(ctx, doc)
	if err != nil {
		orm.Error = err
		return orm
	}

	// Cast InsertedID to primitive.ObjectID
	insertedID, ok := result.InsertedID.(primitive.ObjectID)

	if !ok {
		orm.Error = fmt.Errorf("failed to cast inserted ID to ObjectID")
		return orm
	}

	err = collection.FindOne(ctx, bson.M{"_id": insertedID}).Decode(doc)
	orm.filter = nil
	orm.Error = err
	return orm
}

// Example modification in Save method for ID extraction and error handling
func (orm *MongoORM) Save(doc interface{}) *MongoORM {
	if orm.Error != nil {
		return orm // Halt if there was a previous error
	}

	collectionName := orm.determineCollectionName(doc)
	orm.collection = orm.client.Database(orm.database).Collection(collectionName)

	docVal := reflect.ValueOf(doc)
	if docVal.Kind() == reflect.Ptr {
		docVal = docVal.Elem()
	}

	idField := docVal.FieldByName("ID")
	if !idField.IsValid() || idField.Elem().Interface().(primitive.ObjectID).IsZero() {
		orm.Error = errors.New("document must have a valid ID field of type primitive.ObjectID")
		return orm
	}

	oid := idField.Elem().Interface().(primitive.ObjectID) // Correct ID extraction

	if beforeSave, ok := doc.(interface{ BeforeSave() }); ok {
		beforeSave.BeforeSave()
	}

	_, err := orm.collection.ReplaceOne(orm.ctx, bson.M{"_id": oid}, doc)
	if err != nil {
		orm.Error = err
		return orm
	}
	return orm
}

func (orm *MongoORM) Delete(doc interface{}, id ...string) *MongoORM {

	if len(id) > 0 && id[0] != "" {
		objectId, err := primitive.ObjectIDFromHex(id[0])
		if err != nil {
			orm.Error = err
			return orm
		}
		orm.filter = bson.M{"_id": objectId}
	} else if orm.filter == nil {
		idField := reflect.ValueOf(doc).Elem().FieldByName("ID")
		if !idField.IsValid() || idField.Type() != reflect.TypeOf(primitive.ObjectID{}) {
			orm.Error = errors.New("document must have an ID field of type primitive.ObjectID for deletion")
			return orm
		}
		oid := idField.Interface().(primitive.ObjectID)
		orm.filter = bson.M{"_id": oid}
	}

	collectionName := orm.determineCollectionName(doc)
	collection := orm.client.Database(orm.database).Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if beforeDelete, ok := doc.(interface{ BeforeDelete() }); ok {
		beforeDelete.BeforeDelete()
	}

	result, err := collection.DeleteOne(ctx, orm.filter)

	orm.RowsAffected = uint(result.DeletedCount)
	orm.Error = err
	return orm
}

func (orm *MongoORM) Preload(name string) *MongoORM {
	if orm.PreloadCollections == nil {
		orm.PreloadCollections = make([]string, 0)
	}
	orm.PreloadCollections = append(orm.PreloadCollections, name)
	return orm
}

func (orm *MongoORM) processPreloads(doc interface{}) {
	if len(orm.PreloadCollections) == 0 || orm.Error != nil {
		return
	}

	docValPtr := reflect.ValueOf(doc)
	docType := reflect.TypeOf(doc)

	if docValPtr.Kind() != reflect.Ptr || docValPtr.Elem().Kind() != reflect.Struct {
		orm.Error = errors.New("document must be a pointer to a struct")
		return
	}

	for _, preload := range orm.PreloadCollections {
		field, found := docType.Elem().FieldByName(preload)
		if !found {
			continue
		}

		collectionName := fmt.Sprintf("%ss", strings.ToLower(field.Type.Elem().Name()))

		ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
		defer cancel()

		collection := orm.client.Database(orm.database).Collection(collectionName)

		if field.Type.Kind() == reflect.Slice {

			// Create a slice of the element type that the field holds.
			sliceType := reflect.SliceOf(field.Type.Elem())
			sliceValue := reflect.MakeSlice(sliceType, 0, 0)

			// newDoc should be a pointer to this newly created slice.
			newDoc := reflect.New(sliceType)
			newDoc.Elem().Set(sliceValue)

			docVal := docValPtr.Elem()
			fieldId := docVal.FieldByName("ID")
			oid := fieldId.Elem().Interface().(primitive.ObjectID)

			docFieldName := docType.Elem().Name()
			refField, found := field.Type.Elem().FieldByName(docFieldName)
			if !found {
				return
			}

			refFieldName, found := getForeignKeyFromTag(refField.Tag)

			if !found {
				return
			}

			foreignRef, found := field.Type.Elem().FieldByName(refFieldName)

			if !found {
				return
			}

			foreignRefName := strings.Split(foreignRef.Tag.Get("bson"), ",")[0]
			filter := bson.M{foreignRefName: oid}
			fmt.Println(foreignRefName)
			cursor, err := collection.Find(ctx, filter)
			if err != nil {
				orm.Error = err
				return
			}

			if err := cursor.All(ctx, newDoc.Interface()); err != nil {
				orm.Error = err
				return
			}

			// docVal.FieldByName(preload).Set(newDoc)
			docVal.FieldByName(preload).Set(newDoc.Elem())

		}

		if field.Type.Kind() == reflect.Ptr {

			fieldIdName, found := getForeignKeyFromTag(field.Tag)

			if !found {
				return
			}

			newDoc := reflect.New(field.Type.Elem())

			docVal := docValPtr.Elem()
			fieldId := docVal.FieldByName(fieldIdName)
			oid := fieldId.Interface().(primitive.ObjectID)
			if err := collection.FindOne(ctx, bson.M{"_id": oid}).Decode(newDoc.Interface()); err != nil {
				orm.Error = err
				return
			}
			docVal.FieldByName(preload).Set(newDoc)
		}

	}

	orm.PreloadCollections = nil
}

func (orm *MongoORM) Model(doc interface{}) *MongoORM {
	collectionName := orm.determineCollectionName(doc)
	orm.collection = orm.client.Database(orm.database).Collection(collectionName)
	return orm
}

// Select specifies the fields to be returned in the query results.
func (orm *MongoORM) Select(fields ...string) *MongoORM {
	if orm.Error != nil {
		return orm
	}

	// fields = append(fields)
	projection := bson.M{}
	for _, field := range fields {
		projection[field] = 1
	}
	orm.fields = projection
	return orm
}

// Updates performs an update operation on the document(s) matching the criteria.
func (orm *MongoORM) Updates(updateData interface{}) *MongoORM {
	if orm.Error != nil {
		return orm
	}

	// Convert updateData to a map for easier processing.
	// Assumes updateData is a struct; adjust accordingly if it's already a map.
	updateDataVal := reflect.ValueOf(updateData)
	if updateDataVal.Kind() == reflect.Ptr {
		updateDataVal = updateDataVal.Elem()
	}

	var update primitive.M

	if orm.fields != nil {
		filteredUpdateData := bson.M{}

		for fieldName, include := range orm.fields {
			if include != 1 {
				continue // Skip fields not set to be included.
			}

			fieldVal := updateDataVal.FieldByName(fieldName)

			if fieldVal.IsValid() && fieldVal.Kind() != reflect.Slice {
				field, _ := reflect.TypeOf(updateData).FieldByName(fieldName)
				bsonFieldName := strings.Split(field.Tag.Get("bson"), ",")[0]
				filteredUpdateData[bsonFieldName] = fieldVal.Interface()
			}
		}

		// Proceed with the update using filteredUpdateData.
		update = bson.M{
			"$set": filteredUpdateData,
		}
	} else {
		bsonData, _ := bson.Marshal(updateData)
		var updateDocument bson.M
		err := bson.Unmarshal(bsonData, &updateDocument)

		if err != nil {
			orm.Error = err
			return orm
		}
		update = bson.M{
			"$set": updateDocument,
		}

	}
	idField := updateDataVal.FieldByName("ID")
	oid := idField.Elem().Interface().(primitive.ObjectID)
	orm.filter = bson.M{
		"_id": oid,
	}

	result, err := orm.collection.UpdateOne(orm.ctx, orm.filter, update)
	if err != nil {
		orm.Error = err
	} else {
		orm.UpdateResult = result
	}
	orm.fields = nil
	return orm
}

// Add a method to set the context
func (orm *MongoORM) WithContext(ctx context.Context) *MongoORM {
	orm.ctx = ctx
	return orm
}

func getForeignKeyFromTag(tags reflect.StructTag) (string, bool) {

	for _, option := range strings.Split(tags.Get("gorm"), ",") {
		keyVal := strings.Split(option, ":")
		key := keyVal[0]
		if key == "foreignKey" {
			return keyVal[1], true
		}
	}
	return "", false
}
