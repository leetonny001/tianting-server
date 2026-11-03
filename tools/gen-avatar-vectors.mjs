// 生成 protocol/vectors/avatar.json（PRD-ui F-UI-21 规则 5、FE-210）：占位头像的颜色下标和首字。
// 颜色下标 = FNV-1a-32(UTF-8(id)) mod 8（用户用 userId，群用 groupId）；首字 = 显示名的第一个码点，ASCII 小写字母转大写。
// 用法：node tools/gen-avatar-vectors.mjs   （写文件）；node tools/gen-avatar-vectors.mjs --check（CI：不一致就失败）
import { readFileSync, writeFileSync } from 'node:fs';

function fnv1a32(s) {
  let h = 0x811c9dc5;
  for (const b of new TextEncoder().encode(s)) {
    h ^= b;
    h = Math.imul(h, 0x01000193) >>> 0;
  }
  return h >>> 0;
}
const initial = (name) => {
  const first = [...name.trim()][0] ?? '?';
  return /^[a-z]$/.test(first) ? first.toUpperCase() : first;
};

const ids = ['alice', 'bob', 'carol', 'dave', 'Alice', 'ALICE', 'u1', 'u2', 'user_01', 'user-01', 'zhangsan', 'lisi',
  'g1', 'g2', 'ug-01J0000000', 'weekend-football', 'a', 'Z', '0', '9', '_', '-', 'x'.repeat(64), 'kefu', 'official-01',
  'test_user_with_long_id_0123456789'];
const names = ['bob', 'Bob', '张三', '李四', '😀abc', '👨‍👩‍👧 family', '  alice  ', 'élise', 'Ωmega', '1号', '_hidden', '卡姐', 'ｆｕｌｌ', '?'];

const out = {
  comment: '占位头像共享用例（PRD-ui F-UI-21、AC-UI-21-1、AC-UI-21-2）。colors：id → FNV-1a-32(UTF-8(id)) mod 8；initials：显示名 → 首字（第一个码点，ASCII 小写字母转大写，先去首尾空白）。由 tools/gen-avatar-vectors.mjs 生成，不要手改。',
  colors: ids.map((id) => ({ id, fnv1a32: fnv1a32(id), index: fnv1a32(id) % 8 })),
  initials: names.map((name) => ({ name, initial: initial(name) })),
};
const text = JSON.stringify(out, null, 2) + '\n';
const path = new URL('../protocol/vectors/avatar.json', import.meta.url);
if (process.argv.includes('--check')) {
  if (readFileSync(path, 'utf8') !== text) {
    console.error('protocol/vectors/avatar.json is out of date: run node tools/gen-avatar-vectors.mjs');
    process.exit(1);
  }
} else writeFileSync(path, text);
