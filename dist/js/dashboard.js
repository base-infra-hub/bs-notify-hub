const API_URL = '/web/dashboard';
const REFRESH_INTERVAL = 1000;

const targetTypeMap = { 0: '单用户', 1: '多用户', 2: '广播' };
const senderTypeMap = { 0: '用户', 1: '租户' };

let tenantChart = null;
let msgChart = null;
let refreshTimer = null;
let msgHistory = [];

function initCharts() {
    tenantChart = echarts.init(document.getElementById('tenantChart'));
    msgChart = echarts.init(document.getElementById('msgChart'));

    const commonOption = {
        backgroundColor: 'transparent',
        textStyle: { fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace' },
        tooltip: {
            backgroundColor: 'rgba(17, 24, 39, 0.95)',
            borderColor: 'rgba(56, 189, 248, 0.3)',
            textStyle: { color: '#e5e7eb' }
        }
    };

    tenantChart.setOption({
        ...commonOption,
        grid: { top: 30, right: 30, bottom: 20, left: 80, containLabel: true },
        xAxis: {
            type: 'value',
            axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
            splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } },
            axisLabel: { color: '#9ca3af' }
        },
        yAxis: {
            type: 'category',
            axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
            axisLabel: { color: '#9ca3af' },
            data: []
        },
        series: [{
            type: 'bar',
            data: [],
            itemStyle: {
                borderRadius: [0, 6, 6, 0],
                color: new echarts.graphic.LinearGradient(0, 0, 1, 0, [
                    { offset: 0, color: '#0ea5e9' },
                    { offset: 1, color: '#22d3ee' }
                ])
            },
            label: {
                show: true,
                position: 'right',
                color: '#e5e7eb',
                formatter: '{c} 连接'
            }
        }]
    });

    msgChart.setOption({
        ...commonOption,
        grid: { top: 20, right: 20, bottom: 30, left: 50 },
        xAxis: {
            type: 'category',
            axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
            axisLabel: { color: '#9ca3af', rotate: 30, fontSize: 10 },
            data: []
        },
        yAxis: {
            type: 'value',
            axisLine: { lineStyle: { color: 'rgba(255,255,255,0.1)' } },
            splitLine: { lineStyle: { color: 'rgba(255,255,255,0.05)' } },
            axisLabel: { color: '#9ca3af' }
        },
        series: [{
            type: 'line',
            smooth: true,
            symbol: 'circle',
            symbolSize: 8,
            data: [],
            lineStyle: { color: '#38bdf8', width: 3 },
            itemStyle: { color: '#38bdf8', borderColor: '#fff', borderWidth: 1 },
            areaStyle: {
                color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: 'rgba(56, 189, 248, 0.45)' },
                    { offset: 1, color: 'rgba(56, 189, 248, 0)' }
                ])
            }
        }]
    });

    window.addEventListener('resize', () => {
        tenantChart && tenantChart.resize();
        msgChart && msgChart.resize();
    });
}

async function fetchDashboard() {
    try {
        const res = await fetch(API_URL);
        if (res.status === 401 || res.status === 302) {
            // Session 过期或未登录，停止轮询并跳转登录页
            stopAutoRefresh();
            window.location.href = '/web/login';
            return;
        }
        if (!res.ok) {
            throw new Error(`HTTP ${res.status}: ${res.statusText}`);
        }
        const result = await res.json();
        if (result.code !== 0) {
            throw new Error(result.msg || '获取数据失败');
        }
        render(result.data);
        updateLastTime(true);
    } catch (err) {
        console.error('Dashboard fetch error:', err);
        updateLastTime(false, err.message);
    }
}

function render(data) {
    // Header stats
    setText('uptime', data.uptime || '--');
    setText('goVersion', data.go_version || '--');
    setText('goroutines', formatNumber(data.goroutines));
    setText('memory', `${data.memory_mb ?? '--'} MB`);

    // Health
    setHealth('db', data.db_healthy);
    setHealth('redis', data.redis_healthy);

    // Metrics
    animateNumber('connections', data.connections ?? 0);
    animateNumber('onlineTenants', data.online_tenants ?? 0);
    animateNumber('tenantCount', data.tenant_count ?? 0);
    animateNumber('onlineUsers', data.online_users ?? 0);
    animateNumber('userCount', data.user_count ?? 0);

    // Charts
    updateTenantChart(data.group_stats);
    updateMsgTrend(data.recent_msgs);

    // Message list
    renderRecentMessages(data.recent_msgs);
}

function setText(id, text) {
    const el = document.getElementById(id);
    if (el) el.textContent = text;
}

function setHealth(type, healthy) {
    const dot = document.getElementById(`${type}Dot`);
    const text = document.getElementById(`${type}Text`);
    if (!dot || !text) return;

    dot.classList.remove('online', 'offline');
    if (healthy) {
        dot.classList.add('online');
        text.textContent = '正常';
        text.className = 'font-medium text-primary-400';
    } else {
        dot.classList.add('offline');
        text.textContent = '异常';
        text.className = 'font-medium text-accent-red';
    }
}

function animateNumber(id, target) {
    const el = document.getElementById(id);
    if (!el) return;

    const current = parseInt(el.dataset.value || '0', 10);
    if (current === target) return;
    el.dataset.value = target;

    const duration = 600;
    const start = performance.now();

    function step(now) {
        const progress = Math.min((now - start) / duration, 1);
        const ease = 1 - Math.pow(1 - progress, 3);
        const value = Math.floor(current + (target - current) * ease);
        el.textContent = formatNumber(value);
        if (progress < 1) requestAnimationFrame(step);
        else el.textContent = formatNumber(target);
    }
    requestAnimationFrame(step);
}

function updateTenantChart(groupStats) {
    if (!tenantChart) return;

    if (!groupStats || Object.keys(groupStats).length === 0) {
        tenantChart.setOption({
            yAxis: { data: [] },
            series: [{ data: [] }],
            graphic: [{
                type: 'text',
                left: 'center',
                top: 'middle',
                style: { text: '暂无在线租户', fill: '#6b7280', fontSize: 14 }
            }]
        });
        return;
    }

    const entries = Object.entries(groupStats)
        .sort((a, b) => a[1].conn_count - b[1].conn_count)
        .slice(-10);

    tenantChart.setOption({
        graphic: [],
        yAxis: { data: entries.map(e => e[0]) },
        series: [{ data: entries.map(e => e[1].conn_count) }]
    });
}

function updateMsgTrend(msgs) {
    if (!msgChart) return;

    // 累积消息历史，去重
    if (msgs && msgs.length > 0) {
        const existingIds = new Set(msgHistory.map(m => m.notify_id));
        for (const m of msgs) {
            if (!existingIds.has(m.notify_id)) {
                msgHistory.push({ ...m, time: new Date(m.created_at) });
                existingIds.add(m.notify_id);
            }
        }
        msgHistory.sort((a, b) => a.time - b.time);
        if (msgHistory.length > 20) {
            msgHistory = msgHistory.slice(-20);
        }
    }

    if (msgHistory.length === 0) {
        msgChart.setOption({
            xAxis: { data: [] },
            series: [{ data: [] }],
            graphic: [{
                type: 'text',
                left: 'center',
                top: 'middle',
                style: { text: '暂无消息', fill: '#6b7280', fontSize: 14 }
            }]
        });
        return;
    }

    const timeLabels = msgHistory.map(m => m.time.toLocaleTimeString());
    const counts = msgHistory.map((_, idx) => idx + 1);

    msgChart.setOption({
        graphic: [],
        xAxis: { data: timeLabels },
        series: [{ data: counts }]
    });
}

function renderRecentMessages(msgs) {
    const container = document.getElementById('messageList');
    if (!msgs || msgs.length === 0) {
        container.innerHTML = '<div class="text-center text-gray-500 py-10 text-sm">暂无消息</div>';
        return;
    }

    container.innerHTML = msgs.map(m => {
        const time = m.created_at ? new Date(m.created_at).toLocaleTimeString() : '--';
        const sender = senderTypeMap[m.sender_type] ?? m.sender_type;
        const target = targetTypeMap[m.target_type] ?? m.target_type;
        const senderColor = m.sender_type === 1 ? 'bg-primary-600/30 text-primary-300' : 'bg-primary-500/20 text-primary-400';
        return `
            <div class="p-3 rounded-xl bg-dark-700/50 border border-white/5 hover:border-primary-400/30 transition-colors">
                <div class="flex items-center justify-between mb-1.5">
                    <span class="text-xs text-gray-500">${time}</span>
                    <span class="text-xs px-2 py-0.5 rounded-full ${senderColor}">${sender}</span>
                </div>
                <div class="text-sm text-gray-200 font-medium truncate mb-1.5" title="${escapeHtml(m.title)}">${escapeHtml(m.title)}</div>
                <div class="flex items-center gap-2 text-xs">
                    <span class="text-primary-400">${escapeHtml(m.tenant_id)}</span>
                    <span class="text-gray-600">|</span>
                    <span class="text-gray-400">${target}</span>
                </div>
            </div>
        `;
    }).join('');
}

function updateLastTime(success, msg) {
    const el = document.getElementById('lastUpdate');
    if (!el) return;
    if (success) {
        el.textContent = `最后更新: ${new Date().toLocaleString()}`;
        el.className = '';
    } else {
        el.textContent = `更新失败: ${msg}`;
        el.className = 'text-accent-red';
    }
}

function formatNumber(n) {
    if (n === undefined || n === null || isNaN(n)) return '--';
    return Number(n).toLocaleString();
}

function escapeHtml(text) {
    if (text === undefined || text === null) return '';
    return String(text)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

function startAutoRefresh() {
    stopAutoRefresh();
    fetchDashboard();
    refreshTimer = setInterval(fetchDashboard, REFRESH_INTERVAL);
}

function stopAutoRefresh() {
    if (refreshTimer) {
        clearInterval(refreshTimer);
        refreshTimer = null;
    }
}

window.addEventListener('load', () => {
    initCharts();
    startAutoRefresh();

    // 绑定登出事件
    document.getElementById('logoutBtn')?.addEventListener('click', async () => {
        try {
            const res = await fetch('/web/logout', { method: 'POST' });
            if (res.ok) {
                // 停止自动轮询并返回登录页
                stopAutoRefresh();
                window.location.href = '/web/login';
            } else {
                alert('登出失败，请重试');
            }
        } catch (err) {
            console.error('Logout request error:', err);
            alert('网络错误，请稍后重试');
        }
    });
});
window.addEventListener('beforeunload', stopAutoRefresh);
