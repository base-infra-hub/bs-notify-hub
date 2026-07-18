package repository

import (
	"bs-notify-hub/pkg/lua"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var batchGetUnreadLua = redis.NewScript(lua.BatchGetUnreadScript)

var batchIncrIfExistLua = redis.NewScript(`
    if redis.call("EXISTS", KEYS[1]) == 1 then
        return redis.call("INCR", KEYS[1])
    else
        return -1
    end
`)

var decrWithFloorLua = redis.NewScript(`
    local v = redis.call("DECR", KEYS[1])

    if v < 0 then
        redis.call("SET", KEYS[1], 0)
        return 0
    end

    return v
`)
var sDiffCountLua = redis.NewScript(`
    local total_count = redis.call("SCARD", KEYS[1])

    if total_count == 0 then
        return 0
    end

    local inter_count = redis.call(
        "SINTERCARD",
        2,
        KEYS[1],
        KEYS[2]
    )

    local unread = total_count - inter_count

    if unread < 0 then
        unread = 0
    end

    return unread
`)

type NotifyUnreadCacheRepo struct {
	rdb redis.UniversalClient
}

var (
	unreadCacheOnce sync.Once
	cacheRepo       *NotifyUnreadCacheRepo
)

func NewNotifyUnreadCacheRepo(rdb redis.UniversalClient) *NotifyUnreadCacheRepo {
	unreadCacheOnce.Do(func() {
		cacheRepo = &NotifyUnreadCacheRepo{
			rdb: rdb,
		}
		if err := cacheRepo.loadScripts(context.Background()); err != nil {
			panic(err)
		}
	})
	return cacheRepo
}

// Incr 将 key 的值加 1
func (r *NotifyUnreadCacheRepo) Incr(ctx context.Context, key string) error {
	return r.rdb.Incr(ctx, key).Err()
}

// DecrWithFloor 将 key 的值减 1，但不允许小于 0
func (r *NotifyUnreadCacheRepo) DecrWithFloor(
	ctx context.Context,
	key string,
) (int64, error) {

	return decrWithFloorLua.Run(
		ctx,
		r.rdb,
		[]string{key},
	).Int64()
}

// Set 设置 key 的值
func (r *NotifyUnreadCacheRepo) Set(ctx context.Context, key string, val int64) error {
	return r.rdb.Set(ctx, key, val, 0).Err()
}

// SAdd 添加元素到集合
func (r *NotifyUnreadCacheRepo) SAdd(ctx context.Context, key string, member interface{}) (int64, error) {
	return r.rdb.SAdd(ctx, key, member).Result()
}

// SCard 获取集合元素个数
func (r *NotifyUnreadCacheRepo) SCard(ctx context.Context, key string) (int64, error) {
	return r.rdb.SCard(ctx, key).Result()
}

// SafeReplaceSet 使用 TempKey + Rename 保证 Set 替换的原子性
func (r *NotifyUnreadCacheRepo) SafeReplaceSet(ctx context.Context, key string, ids []string) error {
	if len(ids) == 0 {
		return r.rdb.Del(ctx, key).Err()
	}

	tempKey := fmt.Sprintf("%s:temp:%d", key, time.Now().UnixNano())
	pipe := r.rdb.Pipeline()
	members := make([]interface{}, len(ids))
	for i, id := range ids {
		members[i] = id
	}
	pipe.SAdd(ctx, tempKey, members...)
	pipe.Expire(ctx, tempKey, 10*time.Minute)
	pipe.Rename(ctx, tempKey, key)
	pipe.Persist(ctx, key)
	_, err := pipe.Exec(ctx)
	return err
}

// BatchSetPipeline 批量设置 key 的值
func (r *NotifyUnreadCacheRepo) BatchSetPipeline(ctx context.Context, kv map[string]int64) error {
	if len(kv) == 0 {
		return nil
	}
	pipe := r.rdb.Pipeline()
	for k, v := range kv {
		pipe.Set(ctx, k, v, 0)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// BatchIncrIfExist 批量对存在的 key 进行 incr 操作，不存在的 key 返回 -1
func (r *NotifyUnreadCacheRepo) BatchIncrIfExist(
	ctx context.Context,
	keys []string,
) ([]int, error) {

	if len(keys) == 0 {
		return nil, nil
	}

	execute := func() ([]int, error) {

		pipe := r.rdb.Pipeline()

		cmds := make([]*redis.Cmd, 0, len(keys))

		for _, key := range keys {
			cmds = append(
				cmds,
				batchIncrIfExistLua.Run(
					ctx,
					pipe,
					[]string{key},
				),
			)
		}

		_, err := pipe.Exec(ctx)
		if err != nil {
			return nil, err
		}

		results := make([]int, 0, len(cmds))

		for _, cmd := range cmds {

			val, err := cmd.Int64()
			if err != nil {
				return nil, err
			}

			results = append(results, int(val))
		}

		return results, nil
	}

	results, err := execute()

	if err != nil {

		if !strings.Contains(err.Error(), "NOSCRIPT") {
			return nil, fmt.Errorf(
				"BatchIncrIfExist失败: %w",
				err,
			)
		}

		if loadErr := batchIncrIfExistLua.Load(
			ctx,
			r.rdb,
		).Err(); loadErr != nil {

			return nil, fmt.Errorf(
				"加载Lua失败: %w",
				loadErr,
			)
		}

		return execute()
	}

	return results, nil
}

// Get 获取 key 的值，key 不存在时返回 0
func (r *NotifyUnreadCacheRepo) Get(
	ctx context.Context,
	key string,
) (int64, error) {
	val, err := r.rdb.Get(ctx, key).Int64()

	if errors.Is(err, redis.Nil) {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	return val, nil
}

// SDiffCount 计算两个集合的差集数量
func (r *NotifyUnreadCacheRepo) SDiffCount(
	ctx context.Context,
	totalKey string,
	opKey string,
) (int64, error) {
	return sDiffCountLua.Run(
		ctx,
		r.rdb,
		[]string{
			totalKey,
			opKey,
		},
	).Int64()
}

// BatchGetUnreadDual 批量获取用户双维度未读数
// keys: [pKey1, tKey1, opKey1, pKey2, tKey2, opKey2...] (每3个一组)
func (r *NotifyUnreadCacheRepo) BatchGetUnreadDual(ctx context.Context, totalKey string, keys []string) ([]int64, error) {
	res, err := batchGetUnreadLua.Run(ctx, r.rdb, []string{totalKey}, keys).Int64Slice()
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *NotifyUnreadCacheRepo) SyncTenantBroadcastToOp(ctx context.Context, tenantBcastKey, tenantBcastOpKey string) (int64, error) {
	result, err := r.rdb.SUnionStore(ctx, tenantBcastOpKey, tenantBcastOpKey, tenantBcastKey).Result()
	if err != nil {
		return 0, err
	}
	return result, nil
}
func (r *NotifyUnreadCacheRepo) loadScripts(
	ctx context.Context,
) error {

	if err := batchIncrIfExistLua.Load(ctx, r.rdb).Err(); err != nil {
		return err
	}
	if err := sDiffCountLua.Load(ctx, r.rdb).Err(); err != nil {
		return err
	}
	if err := decrWithFloorLua.Load(ctx, r.rdb).Err(); err != nil {
		return err
	}

	if err := batchGetUnreadLua.Load(ctx, r.rdb).Err(); err != nil {
		return err
	}

	return nil
}
