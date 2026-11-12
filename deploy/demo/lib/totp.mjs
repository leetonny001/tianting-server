// TOTP（RFC 6238，SHA-1、6 位、30 秒），和 console-server 一致。只用 Node 标准库。
//   node lib/totp.mjs <.run 目录> [账号名]    打印后台账号当前的动态码（不带账号名时全部打印）
import { createHmac } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

export const STEP_MS = 30_000;

function base32Decode(text) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = 0;
  let value = 0;
  const out = [];
  for (const c of text.replace(/=+$/, '').toUpperCase()) {
    const i = alphabet.indexOf(c);
    if (i < 0) continue;
    value = (value << 5) | i;
    bits += 5;
    if (bits >= 8) {
      out.push((value >>> (bits - 8)) & 255);
      bits -= 8;
    }
  }
  return Buffer.from(out);
}

export const stepOf = (ms) => Math.floor(ms / STEP_MS);

export function totp(seed, step = stepOf(Date.now())) {
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(step));
  const h = createHmac('sha1', base32Decode(seed)).update(counter).digest();
  const off = h[h.length - 1] & 15;
  return String((h.readUInt32BE(off) & 0x7fffffff) % 1_000_000).padStart(6, '0');
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const [run, who] = process.argv.slice(2);
  const accounts = JSON.parse(readFileSync(join(run, 'console-accounts.json'), 'utf8'));
  const left = Math.ceil((STEP_MS - (Date.now() % STEP_MS)) / 1000);
  const list = accounts.filter((a) => !who || a.username === who || a.label === who);
  if (list.length === 0) {
    console.error(`没有这个后台账号：${who}（有：${accounts.map((a) => a.username).join('、')}）`);
    process.exit(2);
  }
  for (const a of list) console.log(`${a.label.padEnd(4, '　')} ${a.username.padEnd(9)} ${totp(a.totpSecret)}`);
  console.log(`（还有 ${left} 秒换下一个码；同一个码只能用一次，提示动态码错误就等下一个）`);
}
