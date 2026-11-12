// 生成一对 Ed25519 密钥，输出 "base64种子 base64公钥"（console-server 的 CON_IM_SIGNING_KEY 要种子，IM_ADMIN_CALLERS 要公钥）。
// 和 deploy/testenv/genkey.go 做的事一样，这里用 Node 是因为演示脚本别的部分也用 Node。
import { generateKeyPairSync } from 'node:crypto';

const { publicKey, privateKey } = generateKeyPairSync('ed25519');
const seed = privateKey.export({ format: 'der', type: 'pkcs8' }).subarray(-32);
const pub = publicKey.export({ format: 'der', type: 'spki' }).subarray(-32);
console.log(`${seed.toString('base64')} ${pub.toString('base64')}`);
