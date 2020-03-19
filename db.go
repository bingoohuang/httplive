package httplive

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"

	"github.com/sirupsen/logrus"

	"github.com/boltdb/bolt"
)

// OpenDB ...
func OpenDB() *bolt.DB {
	config := &bolt.Options{Timeout: 1 * time.Second} // nolint gomnd
	db, err := bolt.Open(Environments.DBFile, 0600, config)

	if err != nil {
		logrus.Fatalf("fail to open db file %s error %v", Environments.DBFile, err)
	}

	return db
}

// CreateDBBucket ...
func CreateDBBucket() error {
	db := OpenDB()
	defer db.Close()

	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(Environments.DefaultPort))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})

	return err
}

// InitDBValues ...
func InitDBValues() {
	apis := []APIDataModel{
		{
			Endpoint: "/api/token/mobiletoken",
			Method:   http.MethodGet,
			Body: `{
	"array": [
		1,
		2,
		3
	],
	"boolean": true,
	"null": null,
	"number": 123,
	"object": {
		"a": "b",
		"c": "d",
		"e": "f"
	},
	"string": "Hello World"
}`}}

	for _, api := range apis {
		key := CreateEndpointKey(api.Method, api.Endpoint)
		if model, _ := GetEndpoint(key); model == nil {
			_ = SaveEndpoint(api)
		}
	}

	SyncEndpointRouter()
}

func (model *APIDataModel) gobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := gob.NewEncoder(buf).Encode(model)

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func gobDecode(data []byte) (*APIDataModel, error) {
	var model APIDataModel
	err := gob.NewDecoder(bytes.NewBuffer(data)).Decode(&model)

	if err != nil {
		return nil, err
	}

	return &model, nil
}

// SaveEndpoint ...
func SaveEndpoint(model APIDataModel) error {
	if model.Endpoint == "" || model.Method == "" {
		return fmt.Errorf("model endpoint and method could not be empty")
	}

	key := CreateEndpointKey(model.Method, model.Endpoint)

	defer SyncEndpointRouter()

	db := OpenDB()
	defer db.Close()

	err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(Environments.DefaultPort))
		if model.ID <= 0 {
			id, _ := bucket.NextSequence()
			model.ID = int(id)
		}

		enc, err := model.gobEncode()
		if err != nil {
			return fmt.Errorf("could not encode APIDataModel %s: %s", key, err)
		}

		return bucket.Put([]byte(key), enc)
	})

	return err
}

// DeleteEndpoint ...
func DeleteEndpoint(endpointKey string) error {
	if endpointKey == "" {
		return fmt.Errorf("endpointKey")
	}

	defer SyncEndpointRouter()

	db := OpenDB()
	defer db.Close()

	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(Environments.DefaultPort))
		return b.Delete([]byte(endpointKey))
	})
}

// GetEndpoint ...
func GetEndpoint(endpointKey string) (*APIDataModel, error) {
	if endpointKey == "" {
		return nil, fmt.Errorf("endpointKey")
	}

	var model *APIDataModel

	db := OpenDB()
	defer db.Close()

	if err := db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(Environments.DefaultPort))
		k := []byte(endpointKey)
		v := b.Get(k)
		if v != nil {
			model, err = gobDecode(v)
		}
		return err
	}); err != nil {
		logrus.Warnf("Could not get content with key: %s", endpointKey)
		return nil, err
	}

	return model, nil
}

// OrderByID ...
func OrderByID(items map[string]APIDataModel) PairList {
	pl := make(PairList, 0, len(items))

	for k, v := range items {
		pl = append(pl, Pair{k, v})
	}

	sort.Sort(sort.Reverse(pl))

	return pl
}

// nolint gochecknoglobals
var (
	endpointRouter       *httprouter.Router
	endpointRouterLock   sync.Mutex
	endpointRouterServed bool

	boradcastThrottler = MakeThrottle(1 * time.Second)
)

// Throttle ...
type Throttle struct {
	tokenC chan bool
	stopC  chan bool
}

// MakeThrottle ...
func MakeThrottle(duration time.Duration) *Throttle {
	t := &Throttle{
		tokenC: make(chan bool, 1),
		stopC:  make(chan bool, 1),
	}

	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		for {
			select {
			case <-t.stopC:
				return
			case <-ticker.C:
				select {
				case t.tokenC <- true:
				default:
				}
			}
		}
	}()

	return t
}

// Stop ...
func (t *Throttle) Stop() {
	t.stopC <- true
}

// Allow ...
func (t *Throttle) Allow() bool {
	select {
	case <-t.tokenC:
		return true
	default:
		return false
	}
}

// EndpointServeHTTP ...
func EndpointServeHTTP(w http.ResponseWriter, r *http.Request) bool {
	endpointRouterLock.Lock()
	defer endpointRouterLock.Unlock()

	endpointRouterServed = false

	endpointRouter.ServeHTTP(w, r)

	return endpointRouterServed
}

// SyncEndpointRouter ...
func SyncEndpointRouter() {
	endpointRouterLock.Lock()
	defer endpointRouterLock.Unlock()

	endpointRouter = httprouter.New()

	for _, endpoint := range EndpointList() {
		ep := endpoint
		if ep.MimeType == "" {
			b := []byte(ep.Body)
			endpointRouter.Handle(ep.Method, ep.Endpoint,
				func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
					endpointRouterServed = true
					w.WriteHeader(http.StatusOK)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					_, _ = w.Write(b)
				})
		} else {
			endpointRouter.Handle(ep.Method, ep.Endpoint,
				func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
					endpointRouterServed = true
					w.WriteHeader(http.StatusOK)
					reader := bytes.NewReader(ep.FileContent)
					http.ServeContent(w, r, ep.Filename, time.Now(), reader)
				})
		}
	}
}

// EndpointList ...
func EndpointList() []APIDataModel {
	data := make(map[string]APIDataModel)

	db := OpenDB()
	defer db.Close()

	_ = db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(Environments.DefaultPort)).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			model, err := gobDecode(v)
			if err == nil {
				data[string(k)] = *model
			}
		}

		return nil
	})

	pairList := OrderByID(data)
	items := make([]APIDataModel, len(pairList))

	for i, v := range pairList {
		items[i] = v.Value
	}

	return items
}
