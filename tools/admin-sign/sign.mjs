// 天听管理接口签名的 Node.js 参考实现（PRD-admin F-KEY-03），只用标准库。
// 用法：node tools/admin-sign/sign.mjs protocol/vectors/admin-sign.json   # 用测试向量自检
import { createHash, createHmac, createPrivateKey, sign as edSign } from 'node:crypto';
import { readFileSync } from 'node:fs';

/** RFC 3986 编码：只保留 A-Z a-z 0-9 - . _ ~。 */
const rfc3986 = (s) =>
  Array.from(new TextEncoder().encode(s), (b) =>
    /[A-Za-z0-9\-._~]/.test(String.fromCharCode(b)) ? String.fromCharCode(b) : '%' + b.toString(16).toUpperCase().padStart(2, '0'),
  ).join('');

/** 查询串规范化：解码后按键名（相同键按值）排序，再按 RFC 3986 编码。 */
export function canonicalQuery(raw) {
  if (!raw) return '';
  const dec = (s) => decodeURIComponent(s.replace(/\+/g, ' '));
  const pairs = raw
    .split('&')
    .filter(Boolean)
    .map((p) => {
      const i = p.indexOf('=');
      return i < 0 ? [dec(p), ''] : [dec(p.slice(0, i)), dec(p.slice(i + 1))];
    });
  // 按 UTF-8 字节序比较（与 Go 的字符串比较一致）。
  const cmp = (a, b) => Buffer.compare(Buffer.from(a, 'utf8'), Buffer.from(b, 'utf8'));
  pairs.sort((a, b) => cmp(a[0], b[0]) || cmp(a[1], b[1]));
  return pairs.map(([k, v]) => rfc3986(k) + '=' + rfc3986(v)).join('&');
}

/** 待签名串：8 行，\n 分隔。 */
export function stringToSign({ method, path, query = '', timestamp, nonce, body = '', operator = '', requestId = '', targetNode = '' }) {
  const q = canonicalQuery(query);
  const bodyHash = createHash('sha256').update(body, 'utf8').digest('hex');
  return [method.toUpperCase(), q ? `${path}?${q}` : path, timestamp, nonce, bodyHash, operator, requestId, targetNode].join('\n');
}

/** HMAC-SHA256 签名，返回 X-IM-Signature 的值（base64url 无填充）。secret 为原始字节。 */
export const signHmac = (secret, sts) => createHmac('sha256', secret).update(sts, 'utf8').digest('base64url');

/** Ed25519 签名。seed 为 32 字节私钥种子。 */
export function signEd25519(seed, sts) {
  const pkcs8 = Buffer.concat([Buffer.from('302e020100300506032b657004220420', 'hex'), seed]);
  const key = createPrivateKey({ key: pkcs8, format: 'der', type: 'pkcs8' });
  return edSign(null, Buffer.from(sts, 'utf8'), key).toString('base64url');
}

if (process.argv[2]) {
  const { vectors } = JSON.parse(readFileSync(process.argv[2], 'utf8'));
  let failed = 0;
  for (const v of vectors) {
    const sts = stringToSign(v);
    const sig = v.alg === 'hmac-sha256' ? signHmac(Buffer.from(v.secret, 'base64'), sts) : signEd25519(Buffer.from(v.seed, 'base64'), sts);
    if (sts !== v.stringToSign || sig !== v.signature) {
      failed++;
      console.error(`FAIL ${v.name}`);
    }
  }
  console.log(`node: ${vectors.length - failed}/${vectors.length} vectors ok`);
  process.exit(failed ? 1 : 0);
}
