// 运维后台（console-server 运维空间，PRD-ops）的演示账号。由 demo.sh 调用（seed、reset、up、ops-attach），只灌一次（.run/seeded-ops）。
//   DEMO_RUN=<.run 目录> node lib/seed-ops.mjs
// 只用 Node 标准库。账号追加到 .run/console-accounts.json（标 space: "ops"），demo.sh otp 和管理后台账号一起列出。
//
// 运维空间的角色（tianting-console-server internal/perm）：
//   ops_superadmin 只能管账号，看不到总览和节点；ops_engineer 能看总览、节点、配置、数据库，做连接排查和告警静默。
//   授予 ops_engineer 要第二个超管批准，第二个超管只能命令行创建（demo.sh 里 bootstrap 了 ywchaoguan、ywfuhe 两个）。
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { STEP_MS, stepOf, totp } from './totp.mjs';

const RUN = process.env.DEMO_RUN;
if (!RUN) throw new Error('DEMO_RUN is required');
const st = JSON.parse(readFileSync(join(RUN, 'state.json'), 'utf8'));

// 公开仓库里的演示值，和管理后台账号同一个密码（seed.mjs 的 CONSOLE_PASSWORD）。
const PASSWORD = 'Tianting-Demo-Only-2026';
const ENGINEER = { username: 'yunwei', label: '运维值班', role: 'ops_engineer', displayName: '运维值班' };

const log = (...a) => console.log('  ·', ...a);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

class Ops {
  cookie = '';
  csrf = '';
  async call(method, path, body) {
    const headers = { Origin: st.opsOrigin };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (this.cookie) headers.Cookie = this.cookie;
    if (this.csrf) headers['X-CSRF-Token'] = this.csrf;
    const res = await fetch(st.opsOrigin + path, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const set = res.headers.get('set-cookie');
    if (set?.startsWith('__Host-ops_sid=')) this.cookie = set.split(';')[0];
    const text = await res.text();
    const data = text ? JSON.parse(text) : {};
    if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${text}`);
    if (data.csrfToken) this.csrf = data.csrfToken;
    return data;
  }
}

// 同一个账号同一个时间步的码只能用一次。
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

async function activate(username, code) {
  const api = new Ops();
  await api.call('POST', '/console/api/auth/activate', { username, code, newPassword: PASSWORD });
  const setup = await api.call('POST', '/console/api/auth/totp/setup', {});
  const uri = setup.otpauthUri;
  const secret = new URL(uri).searchParams.get('secret');
  const done = await api.call('POST', '/console/api/auth/totp/confirm', { otp: await otpFor(secret) });
  return { api, secret, uri, recoveryCodes: done.recoveryCodes ?? [] };
}

if (existsSync(join(RUN, 'seeded-ops'))) {
  console.log('运维后台的演示账号已经灌过了');
  process.exit(0);
}
console.log('灌运维后台账号：');
const [r1, r2] = st.opsRoots;
const root1 = { username: r1.username, label: '运维超管', role: 'ops_superadmin', ...(await activate(r1.username, r1.code)) };
const root2 = { username: r2.username, label: '运维复核', role: 'ops_superadmin', ...(await activate(r2.username, r2.code)) };

await root1.api.call('POST', '/console/api/auth/step-up', { otp: await otpFor(root1.secret) });
const created = await root1.api.call('POST', '/console/api/accounts', {
  username: ENGINEER.username,
  displayName: ENGINEER.displayName,
  roles: [ENGINEER.role],
});
// 批准后这个账号的旧会话会失效，所以先批准再激活（和 tianting-ops/e2e/helpers.ts 的 provision 一样）。
await root2.api.call('POST', '/console/api/auth/step-up', { otp: await otpFor(root2.secret) });
await root2.api.call('POST', `/console/api/accounts/${created.id}/roles/approve`, { role: ENGINEER.role });
const engineer = { ...ENGINEER, ...(await activate(ENGINEER.username, created.activationCode)) };
log(`运维账号：运维值班(yunwei，ops_engineer，${root1.username} 申请、${root2.username} 批准)、运维超管(${root1.username}、${root2.username})`);

// 冒烟：console-server 每 5 秒经 TLS 探测一次节点，探到了才有版本号。等不到不算失败，页面上稍后会出来。
let versions = [];
for (let i = 0; i < 40 && versions.length === 0; i++) {
  const r = await engineer.api.call('GET', '/console/api/ops/v1/nodes');
  versions = r.data?.versions?.list ?? [];
  if (versions.length === 0) await sleep(500);
}
log(versions.length ? `节点探测正常（TLS），版本 ${versions.join('、')}` : '节点还没有探测结果（稍后在页面上看）');

const file = join(RUN, 'console-accounts.json');
const accounts = existsSync(file) ? JSON.parse(readFileSync(file, 'utf8')).filter((a) => a.space !== 'ops') : [];
for (const s of [engineer, root1, root2]) {
  accounts.push({
    space: 'ops',
    username: s.username,
    label: s.label,
    role: s.role,
    password: PASSWORD,
    totpSecret: s.secret,
    otpauthUri: s.uri,
    recoveryCodes: s.recoveryCodes,
  });
}
writeFileSync(file, JSON.stringify(accounts, null, 2), { mode: 0o600 });
writeFileSync(join(RUN, 'seeded-ops'), JSON.stringify({ at: new Date().toISOString() }), { mode: 0o600 });
console.log('运维后台账号灌好了');
process.exit(0);
