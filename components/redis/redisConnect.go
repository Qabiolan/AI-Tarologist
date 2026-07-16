package redis

import (
	"bot/components/models"
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	configReader "bot/config"

	redis "github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client *redis.Client
}

type memoryStore struct {
	mu    sync.RWMutex
	data  map[string]string
	users map[string]models.User
}

var memStore = &memoryStore{
	data:  make(map[string]string),
	users: make(map[string]models.User),
}

var config = configReader.Readconfig()

func NewClient() *RedisClient {
	// If Redis is not configured, use in-memory store
	if config.REDIS_ADDR == "" {
		return &RedisClient{
			client: nil,
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     config.REDIS_ADDR,
		Password: config.REDIS_PASSWORD,
		DB:       0,
	})
	return &RedisClient{
		client: rdb,
	}
}

func (r *RedisClient) Setter(ctx context.Context, key string, value string, expiration time.Duration) error {
	if r.client == nil {
		memStore.mu.Lock()
		defer memStore.mu.Unlock()
		memStore.data[key] = value
		return nil
	}
	return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *RedisClient) Getter(ctx context.Context, key string) (string, error) {
	if r.client == nil {
		memStore.mu.RLock()
		defer memStore.mu.RUnlock()
		if val, ok := memStore.data[key]; ok {
			return val, nil
		}
		return "", nil
	}
	return r.client.Get(ctx, key).Result()
}

func (r *RedisClient) SetNewUser(
	ctx context.Context,
	ID int,
	Username string,
	isPremium bool,
	Info string,
	PaymentHistory map[string]string,
	expiration time.Duration,
) error {
	new_user, err := models.NewUser(ID, Username, isPremium, Info, nil)
	if err != nil {
		panic(err)
	}

	if r.client == nil {
		memStore.mu.Lock()
		defer memStore.mu.Unlock()
		memStore.users[strconv.Itoa(ID)] = new_user
		return nil
	}
	return r.client.Set(ctx, strconv.Itoa(ID), new_user, expiration).Err()
}

func (r *RedisClient) UpdateFieldUser(
	ctx context.Context,
	ID int,
	field string,
	value interface{},
	expiration time.Duration,
) error {
	key := strconv.Itoa(ID)

	if r.client == nil {
		memStore.mu.Lock()
		defer memStore.mu.Unlock()

		user, ok := memStore.users[key]
		if !ok {
			user = models.User{ID: ID, Username: "unknown"}
		}

		switch field {
		case "username":
			if v, ok := value.(string); ok {
				user.Username = v
			}
		case "isPremium":
			if v, ok := value.(bool); ok {
				user.IsPremium = v
			}
		case "info":
			if v, ok := value.(string); ok {
				user.Info = v
			}
		case "paymentHistory":
			if v, ok := value.(map[string]string); ok {
				user.PaymentHistory = v
			}
		}

		memStore.users[key] = user
		return nil
	}

	var user models.User
	userData, err := r.client.Get(ctx, key).Result()
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal([]byte(userData), &user); err != nil {
		return err
	}

	switch field {
	case "username":
		if username, ok := value.(string); ok {
			user.Username = username
		} else {
			return err
		}
	case "isPremium":
		if isPremium, ok := value.(bool); ok {
			user.IsPremium = isPremium
		} else {
			return err
		}
	case "info":
		if info, ok := value.(string); ok {
			user.Info = info
		} else {
			return err
		}
	case "paymentHistory":
		if paymentHistory, ok := value.(map[string]string); ok {
			user.PaymentHistory = paymentHistory
		} else {
			return err
		}
	default:
		return err
	}

	updatedUserData, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, updatedUserData, expiration).Err()
}

func (r *RedisClient) ReadUser(ctx context.Context, id int) (models.User, error) {
	key := strconv.Itoa(id)

	if r.client == nil {
		memStore.mu.RLock()
		defer memStore.mu.RUnlock()
		if user, ok := memStore.users[key]; ok {
			return user, nil
		}
		return models.User{}, nil
	}

	var user models.User
	userData, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return models.User{}, err
	}
	if err := json.Unmarshal([]byte(userData), &user); err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (r *RedisClient) GetUser(ctx context.Context, key string) (models.User, error) {
	if r.client == nil {
		memStore.mu.RLock()
		defer memStore.mu.RUnlock()
		if user, ok := memStore.users[key]; ok {
			return user, nil
		}
		return models.User{}, nil
	}

	var user models.User
	userData, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return models.User{}, err
	}
	if err := json.Unmarshal([]byte(userData), &user); err != nil {
		return models.User{}, err
	}
	return user, nil
}
