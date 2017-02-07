// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package storage

import (
	"sort"
	"strings"

	"gopkg.in/redis.v5"
)

// RedisQueueStore stores queues in Redis
type RedisQueueStore struct {
	prefix string
	client *redis.Client
}

// NewRedisQueueStore creates a new RedisQueueStore
func NewRedisQueueStore(client *redis.Client, prefix string) *RedisQueueStore {
	if !strings.HasSuffix(prefix, ":") {
		prefix += ":"
	}
	return &RedisQueueStore{
		client: client,
		prefix: prefix,
	}
}

// GetAll returns all results for the given keys, prepending the prefix to the keys if necessary
func (s *RedisQueueStore) GetAll(keys []string, options *ListOptions) (map[string][]string, error) {
	if len(keys) == 0 {
		return map[string][]string{}, nil
	}

	for i, key := range keys {
		if !strings.HasPrefix(key, s.prefix) {
			keys[i] = s.prefix + key
		}
	}

	sort.Strings(keys)

	selectedKeys := selectKeys(keys, options)
	if len(selectedKeys) == 0 {
		return map[string][]string{}, nil
	}

	pipe := s.client.Pipeline()
	defer pipe.Close()

	// Add all commands to pipeline
	cmds := make(map[string]*redis.StringSliceCmd)
	for _, key := range selectedKeys {
		cmds[key] = pipe.LRange(key, 0, -1)
	}

	// Execute pipeline
	_, err := pipe.Exec()
	if err != nil {
		return nil, err
	}

	// Get all results from pipeline
	data := make(map[string][]string)
	for key, cmd := range cmds {
		res, err := cmd.Result()
		if err == nil {
			data[strings.TrimPrefix(key, s.prefix)] = res
		}
	}

	return data, nil
}

// List all results matching the selector, prepending the prefix to the selector if necessary
func (s *RedisQueueStore) List(selector string, options *ListOptions) (map[string][]string, error) {
	if selector == "" {
		selector = "*"
	}
	if !strings.HasPrefix(selector, s.prefix) {
		selector = s.prefix + selector
	}
	var allKeys []string
	var cursor uint64
	for {
		keys, next, err := s.client.Scan(cursor, selector, 0).Result()
		if err != nil {
			return nil, err
		}
		allKeys = append(allKeys, keys...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return s.GetAll(allKeys, options)
}

// Get one result, prepending the prefix to the key if necessary
func (s *RedisQueueStore) Get(key string) (res []string, err error) {
	if !strings.HasPrefix(key, s.prefix) {
		key = s.prefix + key
	}
	res, err = s.client.LRange(key, 0, -1).Result()
	if err == redis.Nil {
		return res, nil
	}
	return res, err
}

// Length gets the size of a queue, prepending the prefix to the key if necessary
func (s *RedisQueueStore) Length(key string) (int, error) {
	if !strings.HasPrefix(key, s.prefix) {
		key = s.prefix + key
	}
	res, err := s.client.LLen(key).Result()
	if err == redis.Nil {
		return int(res), nil
	}
	return int(res), err
}

// AddFront adds one or more values to the front of the queue, prepending the prefix to the key if necessary
// If you add AddFront("value1", "value2") to an empty queue, then the Next(key) will return "value2".
func (s *RedisQueueStore) AddFront(key string, values ...string) error {
	if !strings.HasPrefix(key, s.prefix) {
		key = s.prefix + key
	}
	valuesI := make([]interface{}, len(values))
	for i, v := range values {
		valuesI[i] = v
	}
	return s.client.LPush(key, valuesI...).Err()
}

// AddEnd adds one or more values to the end of the queue, prepending the prefix to the key if necessary
// If you add AddEnd("value1", "value2") to an empty queue, then the Next(key) will return "value1".
func (s *RedisQueueStore) AddEnd(key string, values ...string) error {
	if !strings.HasPrefix(key, s.prefix) {
		key = s.prefix + key
	}
	valuesI := make([]interface{}, len(values))
	for i, v := range values {
		valuesI[i] = v
	}
	return s.client.RPush(key, valuesI...).Err()
}

// Next the first element from the queue, prepending the prefix to the key if necessary
func (s *RedisQueueStore) Next(key string) (string, error) {
	if !strings.HasPrefix(key, s.prefix) {
		key = s.prefix + key
	}
	res, err := s.client.LPop(key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return res, err
}

// Delete the entire queue
func (s *RedisQueueStore) Delete(key string) error {
	if !strings.HasPrefix(key, s.prefix) {
		key = s.prefix + key
	}
	return s.client.Del(key).Err()
}
