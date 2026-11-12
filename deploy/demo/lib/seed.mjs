// 演示数据：账号、好友、群、历史消息（文字、图片、文件、撤回、引用、@、@所有人）、置顶和免打扰、敏感词、举报、后台账号。
// 由 demo.sh 调用（seed、reset、up），只灌一次：已经灌过（.run/seeded 存在）就退出，要重来用 demo.sh reset。
//   DEMO_RUN=<.run 目录> DEMO_SDK=<tianting-sdk-ts 目录> node lib/seed.mjs
// 只用 Node 标准库、本仓库的 tools/admin-sign/sign.mjs 和同级目录 tianting-sdk-ts 的构建产物（dist），不加任何依赖。
// 参数走环境变量不走 argv：sign.mjs 在 argv[2] 存在时会当成测试向量文件去读。
import { randomBytes } from 'node:crypto';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { signHmac, stringToSign } from '../../../tools/admin-sign/sign.mjs';
import { agendaPdf, landscape } from './media.mjs';
import { STEP_MS, stepOf, totp } from './totp.mjs';

const RUN = process.env.DEMO_RUN;
const SDK = process.env.DEMO_SDK;
if (!RUN || !SDK) throw new Error('DEMO_RUN and DEMO_SDK are required');
const st = JSON.parse(readFileSync(join(RUN, 'state.json'), 'utf8'));
const { IMClient, FileClient, StaticTokenAuth, c2cConvId, groupConvId } = await import(
  pathToFileURL(join(SDK, 'dist/index.js')).href
);

// ---- 演示账号（公开仓库里的演示值：只给本机演示环境用，和任何真实密码都不同） ----
export const IM_PASSWORD = 'tt-demo-2026';
export const CONSOLE_PASSWORD = 'Tianting-Demo-Only-2026';
const USERS = [
  { id: 'xiaoming', nick: '小明', role: 'H5 演示账号' },
  { id: 'xiaohong', nick: '小红', role: 'Android 演示账号' },
  { id: 'xiaogang', nick: '小刚', role: 'iOS 演示账号' },
  { id: 'xiaoli', nick: '小丽', role: '好友' },
  { id: 'awei', nick: '阿伟', role: '好友' },
  { id: 'lurenjia', nick: '路人甲', role: '陌生人（不是任何人的好友）' },
];
const BLOCKED_WORD = '演示违禁词';
const LOBBY = 'demo-lobby';
const STAFF = [
  { username: 'yunying', label: '运营', role: 'operator', displayName: '演示运营' },
  { username: 'shenhe', label: '审核', role: 'moderator', displayName: '演示审核' },
  { username: 'kefu', label: '客服', role: 'support', displayName: '演示客服' },
];

const log = (...a) => console.log('  ·', ...a);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function until(fn, what, timeout = 15_000) {
  const deadline = Date.now() + timeout;
  for (;;) {
    const v = fn();
    if (v) return v;
    if (Date.now() > deadline) throw new Error(`等待超时：${what}`);
    await sleep(25);
  }
}

// ---- im-server 管理接口（调用方签名，protocol/ADMIN.md） ----
async function admin(method, path, body) {
  const raw = body === undefined ? '' : JSON.stringify(body);
  const [p, query = ''] = path.split('?');
  const timestamp = String(Date.now());
  const nonce = randomBytes(18).toString('base64url');
  const operator = 'demo-seed';
  const sts = stringToSign({ method, path: p, query, timestamp, nonce, body: raw, operator });
  const res = await fetch(st.imAdmin + path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      'X-IM-Caller': st.seedCaller,
      'X-IM-Timestamp': timestamp,
      'X-IM-Nonce': nonce,
      'X-IM-Operator': operator,
      'X-IM-Signature': signHmac(Buffer.from(st.seedSecret, 'base64'), sts),
    },
    body: raw || undefined,
  });
  const text = await res.text();
  if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${text}`);
  return text ? JSON.parse(text) : {};
}

// ---- 客户端 HTTP 接口 ----
async function client(method, path, body, token) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(st.imHttp + path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await res.text();
  if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${text}`);
  return text ? JSON.parse(text) : {};
}

// ---- console-server（Cookie 会话 + CSRF，和 tianting-admin/e2e/helpers.ts 的 Api 一样） ----
class Console {
  cookie = '';
  csrf = '';
  async call(method, path, body) {
    const headers = { Origin: st.consoleOrigin };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (this.cookie) headers.Cookie = this.cookie;
    if (this.csrf) headers['X-CSRF-Token'] = this.csrf;
    const res = await fetch(st.consoleOrigin + path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const set = res.headers.get('set-cookie');
    if (set?.startsWith('__Host-adm_sid=')) this.cookie = set.split(';')[0];
    const text = await res.text();
    const data = text ? JSON.parse(text) : {};
    if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${text}`);
    if (data.csrfToken) this.csrf = data.csrfToken;
    return data;
  }
}

// TOTP 同一个时间步只能用一次（console 记最后用过的步）：同一个账号第二次要码时等到下一步。
const usedStep = new Map();
async function otpFor(secret) {
  let step = stepOf(Date.now());
  const last = usedStep.get(secret);
  if (last !== undefined && step <= last) {
    const wait = (last + 1) * STEP_MS - Date.now() + 300;
    log(`等下一个动态码（${Math.ceil(wait / 1000)} 秒）`);
    await sleep(wait);
    step = stepOf(Date.now());
  }
  usedStep.set(secret, step);
  return totp(secret, step);
}

async function activateStaff(username, code) {
  const api = new Console();
  await api.call('POST', '/console/api/auth/activate', { username, code, newPassword: CONSOLE_PASSWORD });
  const setup = await api.call('POST', '/console/api/auth/totp/setup', {});
  const uri = setup.otpauthUri;
  const secret = new URL(uri).searchParams.get('secret');
  const done = await api.call('POST', '/console/api/auth/totp/confirm', { otp: await otpFor(secret) });
  return { api, secret, uri, recoveryCodes: done.recoveryCodes ?? [] };
}

// ---- 主流程 ----
if (existsSync(join(RUN, 'seeded'))) {
  console.log('演示数据已经灌过了；要清空重灌用 demo.sh reset');
  process.exit(0);
}
const t0 = Date.now();

console.log('灌演示数据：');
const { invites } = await admin('POST', '/admin/invites', {
  count: 1,
  maxUses: 50,
  expiresAt: Date.now() + 30 * 86400_000,
  note: '演示环境',
});
const inviteCode = invites[0].code;
log('邀请码已生成');

const sessions = {};
for (const u of USERS) {
  const r = await client('POST', '/v1/account/register', {
    username: u.id,
    password: IM_PASSWORD,
    nickname: u.nick,
    inviteCode,
    platform: 'web',
    deviceName: '演示数据脚本',
  });
  // 新账号的恢复码：确认已保存，App 登录时就不会先弹恢复码页面。
  if (r.recoveryCodeGen) await client('POST', '/v1/account/recovery-code/ack', { gen: r.recoveryCodeGen }, r.accessToken);
  sessions[u.id] = r;
}
log(`注册 ${USERS.length} 个账号：${USERS.map((u) => `${u.nick}(${u.id})`).join('、')}`);

const friends = ['xiaoming', 'xiaohong', 'xiaogang', 'xiaoli', 'awei'];
const pairs = [];
for (let i = 0; i < friends.length; i++)
  for (let j = i + 1; j < friends.length; j++) pairs.push({ a: friends[i], b: friends[j] });
await admin('POST', '/admin/friends/import', { pairs });
log(`好友关系 ${pairs.length} 对（路人甲不是任何人的好友）`);

await admin('POST', '/admin/moderation/words', { word: BLOCKED_WORD });
log(`敏感词：${BLOCKED_WORD}`);

await admin('POST', '/admin/groups', {
  groupId: LOBBY,
  name: '天听演示大群',
  members: USERS.map((u) => u.id),
  historyVisible: true,
});
log('托管群：天听演示大群（后台建的，没有群主）');

// ---- IM 客户端（SDK） ----
const im = {};
for (const u of USERS) {
  const token = sessions[u.id].accessToken;
  const files = new FileClient({ auth: new StaticTokenAuth(st.imHttp, token) });
  const c = new IMClient({ url: st.imWs, browserEvents: false, files, platform: 'web' });
  c.connect(token);
  im[u.id] = c;
}
for (const u of USERS) await until(() => im[u.id].state === 'connected', `${u.id} 连上`);
await Promise.all(USERS.map((u) => im[u.id].refreshSocial()));

async function send(from, convId, text, opts) {
  const r = im[from].send(convId, text, opts);
  if (!r.ok) throw new Error(`send ${from}: ${JSON.stringify(r)}`);
  const m = await until(
    () => im[from].messages(convId).find((x) => x.clientMsgId === r.message.clientMsgId && x.status === 'sent'),
    `${from} 发出 ${text}`,
  );
  await sleep(150); // 时间戳错开，各端显示的顺序和这里一致
  return m;
}

async function sendImage(from, convId, img) {
  const local = im[from].sendImage(convId, img);
  const m = await until(
    () => im[from].messages(convId).find((x) => x.clientMsgId === local.clientMsgId && x.status === 'sent'),
    `${from} 发出图片`,
    30_000,
  );
  await sleep(150);
  return m;
}

async function sendFile(from, convId, file) {
  const local = im[from].sendFile(convId, file);
  const m = await until(
    () => im[from].messages(convId).find((x) => x.clientMsgId === local.clientMsgId && x.status === 'sent'),
    `${from} 发出文件`,
    30_000,
  );
  await sleep(150);
  return m;
}

// 陌生人的好友申请：小明的"新的朋友"里有一条待处理。
await im.lurenjia.applyFriend('xiaoming', { message: '你好，我是路人甲，想加你为好友' });
log('路人甲 → 小明 的好友申请（待处理）');

const { groupId: hikeId } = await im.xiaoming.createGroup({
  name: '周末爬山小队',
  members: ['xiaohong', 'xiaogang', 'xiaoli'],
});
const hike = groupConvId(hikeId);
const lobby = groupConvId(LOBBY);
log('自建群：周末爬山小队（群主小明）');

const mh = c2cConvId('xiaoming', 'xiaohong');
const mg = c2cConvId('xiaoming', 'xiaogang');
const hg = c2cConvId('xiaohong', 'xiaogang');
const ml = c2cConvId('xiaoming', 'xiaoli');

// 托管群
await send('awei', lobby, '欢迎大家来到天听演示大群 🎉');
await send('xiaoli', lobby, '大家好～');
const spam = await send('lurenjia', lobby, '加我领红包！私聊发链接，名额有限');
await send('xiaogang', lobby, '这是什么广告……');

// 小明 ↔ 小红：文字、图片、文件、引用、撤回
const first = await send('xiaohong', mh, '小明，周末有空吗？');
await send('xiaoming', mh, '有空！想去爬山吗？');
await sendImage('xiaoming', mh, landscape('dusk'));
await send('xiaoming', mh, '上次拍的，日落的时候特别好看');
await send('xiaohong', mh, '哇，真好看！我把行程安排发你');
await sendFile('xiaohong', mh, { data: agendaPdf(), name: '周末行程安排.pdf', mime: 'application/pdf' });
await send('xiaoming', mh, '周六早上八点出发？', { quote: { msgId: first.msgId } });
const oops = await send('xiaoming', mh, '这条发错了，马上撤回');
await im.xiaoming.recall(mh, oops.msgId);
await sleep(150);
await send('xiaohong', mh, '好呀，不见不散 👍');
log('小明 ↔ 小红：文字、图片、文件、引用、撤回');

// 小明 ↔ 小刚、小红 ↔ 小刚、小明 ↔ 小丽
await send('xiaogang', mg, '明哥，下午的分享会几点开始？');
await send('xiaoming', mg, '三点，在大会议室');
await send('xiaogang', mg, '收到，我提前十分钟到');
await send('xiaohong', hg, '小刚，周末记得带相机');
await send('xiaogang', hg, '放心，电池已经充好了');
await send('xiaoli', ml, '明天的资料我整理好了，放在群文件里');
await send('xiaoming', ml, '辛苦啦');

// 爬山小队：图片、@、引用、@所有人
await send('xiaoming', hike, '大家好，这个群用来约周末爬山 ⛰️');
await send('xiaoli', hike, '报名 +1');
await sendImage('xiaogang', hike, landscape('morning'));
await send('xiaogang', hike, '这是路线起点的照片');
const ask = await send('xiaoming', hike, '@小红 你负责带零食哈', { mentions: { userIds: ['xiaohong'] } });
await send('xiaohong', hike, '没问题！', { quote: { msgId: ask.msgId } });
await send('xiaoming', hike, '@所有人 周六早上 8 点地铁站集合，别迟到', { mentions: { all: true } });
log('周末爬山小队：图片、@小红、引用、@所有人');

// 会话设置（每个人自己的）：置顶一个、免打扰一个
await im.xiaoming.setConvPinned(mh, true);
await im.xiaoming.setConvNotifyOff(lobby, true);
await im.xiaohong.setConvPinned(hike, true);
await im.xiaohong.setConvNotifyOff(lobby, true);
await im.xiaogang.setConvPinned(mg, true);
await im.xiaogang.setConvNotifyOff(lobby, true);
log('置顶：小明→小红、小红→爬山小队、小刚→小明；免打扰：三个人都把演示大群设成免打扰');

// 举报（待处理），后台举报页按对象聚合。小红、小刚（两台手机）留着不举报那条广告：
// 同一个人 24 小时内重复举报同一对象只算一次，现场从手机上举报，次数才会从 3 变成 4。
const reports = [];
for (const [who, reason] of [
  ['xiaoming', 'spam'],
  ['xiaoli', 'fraud'],
  ['awei', 'spam'],
]) {
  reports.push(await im[who].report({ targetType: 'message', convId: lobby, msgIds: [spam.msgId], reason }));
}
reports.push(await im.awei.report({ targetType: 'user', userId: 'lurenjia', reason: 'spam', note: '在群里发广告' }));
reports.push(await im.xiaoli.report({ targetType: 'user', userId: 'lurenjia', reason: 'fraud' }));
reports.push(await im.awei.report({ targetType: 'group', groupId: LOBBY, reason: 'other', note: '群里有人发广告' }));
log(`举报 ${reports.length} 条（路人甲的广告消息 3 条、路人甲本人 2 条、演示大群 1 条），都是待处理`);

for (const c of Object.values(im)) c.destroy();
// 灌数据用的会话退出，App 的"登录设备"里不会留下"演示数据脚本"。
for (const u of USERS) {
  await client('POST', '/v1/account/logout', { refreshToken: sessions[u.id].refreshToken }, sessions[u.id].accessToken).catch(
    () => undefined,
  );
}

// ---- 后台账号 ----
const root = await activateStaff(st.rootUsername, st.rootActivationCode);
await root.api.call('POST', '/console/api/auth/step-up', { otp: await otpFor(root.secret) });
const staff = [{ username: st.rootUsername, label: '超管', role: 'super_admin', ...root }];
for (const s of STAFF) {
  const created = await root.api.call('POST', '/console/api/accounts', {
    username: s.username,
    displayName: s.displayName,
    roles: [s.role],
  });
  staff.push({ ...s, ...(await activateStaff(s.username, created.activationCode)) });
}
log(`后台账号：${staff.map((s) => `${s.label}(${s.username})`).join('、')}`);

writeFileSync(
  join(RUN, 'console-accounts.json'),
  JSON.stringify(
    staff.map((s) => ({
      username: s.username,
      label: s.label,
      role: s.role,
      password: CONSOLE_PASSWORD,
      totpSecret: s.secret,
      otpauthUri: s.uri,
      recoveryCodes: s.recoveryCodes,
    })),
    null,
    2,
  ),
  { mode: 0o600 },
);
writeFileSync(
  join(RUN, 'seeded'),
  JSON.stringify({ at: new Date().toISOString(), users: USERS, password: IM_PASSWORD, hikeGroupId: hikeId }, null, 2),
  { mode: 0o600 },
);
console.log(`演示数据灌好了（${Math.round((Date.now() - t0) / 1000)} 秒）`);
process.exit(0);
