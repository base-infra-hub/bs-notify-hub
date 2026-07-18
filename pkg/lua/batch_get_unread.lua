--个人未读数 = 用户类发送的未读通知数
--租户未读数 = 租户类发送的未读通知数 + [租户广播总数 -(租户广播总数 filter out 用户已操作的广播数)]

local total_broadcast_key = KEYS[1]
local total_count = redis.call("SCARD", total_broadcast_key)
local res = {}

for i = 1, #ARGV, 3 do
    local p_key = ARGV[i]
    local t_key = ARGV[i + 1]
    local op_key = ARGV[i + 2]
    local p_val = tonumber(redis.call("GET", p_key) or 0)
    local t_point_val = tonumber(redis.call("GET", t_key) or 0)

    local inter_count = 0
    if total_count > 0 then
        inter_count = redis.call("SINTERCARD", 2, total_broadcast_key, op_key)
    end

    table.insert(res, p_val)
    table.insert(res, t_point_val + (total_count - inter_count))
end

return res