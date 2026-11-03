// 天听管理接口签名的 Java 参考实现（PRD-admin F-KEY-03），只用 JDK（15+）标准库。
// 用法：java tools/admin-sign/AdminSign.java protocol/vectors/admin-sign.json   # 用测试向量自检
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyFactory;
import java.security.MessageDigest;
import java.security.PrivateKey;
import java.security.Signature;
import java.security.spec.EdECPrivateKeySpec;
import java.security.spec.NamedParameterSpec;
import java.util.ArrayList;
import java.util.Base64;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

public class AdminSign {
    /** RFC 3986 编码：只保留 A-Z a-z 0-9 - . _ ~。 */
    static String rfc3986(String s) {
        StringBuilder b = new StringBuilder();
        for (byte x : s.getBytes(StandardCharsets.UTF_8)) {
            int c = x & 0xff;
            if (c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~') {
                b.append((char) c);
            } else {
                b.append('%').append(String.format("%02X", c));
            }
        }
        return b.toString();
    }

    /** 查询串规范化：解码后按键名（相同键按值）以 UTF-8 字节序排序，再按 RFC 3986 编码。 */
    static String canonicalQuery(String raw) {
        if (raw == null || raw.isEmpty()) return "";
        List<String[]> pairs = new ArrayList<>();
        for (String p : raw.split("&")) {
            if (p.isEmpty()) continue;
            int i = p.indexOf('=');
            String k = i < 0 ? p : p.substring(0, i), v = i < 0 ? "" : p.substring(i + 1);
            pairs.add(new String[] {java.net.URLDecoder.decode(k, StandardCharsets.UTF_8), java.net.URLDecoder.decode(v, StandardCharsets.UTF_8)});
        }
        pairs.sort((a, b) -> {
            int c = java.util.Arrays.compareUnsigned(a[0].getBytes(StandardCharsets.UTF_8), b[0].getBytes(StandardCharsets.UTF_8));
            return c != 0 ? c : java.util.Arrays.compareUnsigned(a[1].getBytes(StandardCharsets.UTF_8), b[1].getBytes(StandardCharsets.UTF_8));
        });
        List<String> out = new ArrayList<>();
        for (String[] kv : pairs) out.add(rfc3986(kv[0]) + "=" + rfc3986(kv[1]));
        return String.join("&", out);
    }

    /** 待签名串：8 行，\n 分隔。 */
    static String stringToSign(String method, String path, String query, String ts, String nonce, String body,
                               String operator, String requestId, String targetNode) throws Exception {
        String q = canonicalQuery(query);
        byte[] hash = MessageDigest.getInstance("SHA-256").digest(body.getBytes(StandardCharsets.UTF_8));
        return String.join("\n", method.toUpperCase(), q.isEmpty() ? path : path + "?" + q, ts, nonce,
                HexFormat.of().formatHex(hash), operator, requestId, targetNode);
    }

    /** HMAC-SHA256 签名，返回 X-IM-Signature（base64url 无填充）。 */
    static String signHmac(byte[] secret, String sts) throws Exception {
        Mac mac = Mac.getInstance("HmacSHA256");
        mac.init(new SecretKeySpec(secret, "HmacSHA256"));
        return Base64.getUrlEncoder().withoutPadding().encodeToString(mac.doFinal(sts.getBytes(StandardCharsets.UTF_8)));
    }

    /** Ed25519 签名，seed 为 32 字节私钥种子。 */
    static String signEd25519(byte[] seed, String sts) throws Exception {
        PrivateKey key = KeyFactory.getInstance("Ed25519").generatePrivate(new EdECPrivateKeySpec(NamedParameterSpec.ED25519, seed));
        Signature s = Signature.getInstance("Ed25519");
        s.initSign(key);
        s.update(sts.getBytes(StandardCharsets.UTF_8));
        return Base64.getUrlEncoder().withoutPadding().encodeToString(s.sign());
    }

    // ---- 自检：读取测试向量（极简 JSON 解析，只支持向量文件的格式：对象数组、字符串值）----

    public static void main(String[] args) throws Exception {
        String json = Files.readString(Path.of(args[0]));
        List<Map<String, String>> vectors = parseVectors(json.substring(json.indexOf("\"vectors\"")));
        int failed = 0;
        for (Map<String, String> v : vectors) {
            String sts = stringToSign(v.get("method"), v.get("path"), v.get("query"), v.get("timestamp"), v.get("nonce"),
                    v.get("body"), v.get("operator"), v.get("requestId"), v.get("targetNode"));
            String sig = v.get("alg").equals("hmac-sha256")
                    ? signHmac(Base64.getDecoder().decode(v.get("secret")), sts)
                    : signEd25519(Base64.getDecoder().decode(v.get("seed")), sts);
            if (!sts.equals(v.get("stringToSign")) || !sig.equals(v.get("signature"))) {
                failed++;
                System.err.println("FAIL " + v.get("name"));
            }
        }
        System.out.println("java: " + (vectors.size() - failed) + "/" + vectors.size() + " vectors ok");
        System.exit(failed == 0 ? 0 : 1);
    }

    static int pos;

    static List<Map<String, String>> parseVectors(String s) {
        List<Map<String, String>> out = new ArrayList<>();
        pos = s.indexOf('[');
        while (true) {
            int obj = s.indexOf('{', pos), end = s.indexOf(']', pos);
            // 向量对象之间没有 ]；最后一个对象之后找不到 {，结束。
            if (obj < 0 || (end >= 0 && end < obj)) break;
            pos = obj + 1;
            Map<String, String> m = new LinkedHashMap<>();
            while (true) {
                skipWs(s);
                if (s.charAt(pos) == '}') { pos++; break; }
                if (s.charAt(pos) == ',') { pos++; continue; }
                String k = readString(s);
                skipWs(s);
                pos++; // ':'
                skipWs(s);
                m.put(k, readString(s));
            }
            out.add(m);
        }
        return out;
    }

    static void skipWs(String s) { while (Character.isWhitespace(s.charAt(pos))) pos++; }

    static String readString(String s) {
        StringBuilder b = new StringBuilder();
        pos++; // 开头的 "
        while (true) {
            char c = s.charAt(pos++);
            if (c == '"') return b.toString();
            if (c != '\\') { b.append(c); continue; }
            char e = s.charAt(pos++);
            switch (e) {
                case 'n' -> b.append('\n');
                case 't' -> b.append('\t');
                case 'u' -> { b.append((char) Integer.parseInt(s.substring(pos, pos + 4), 16)); pos += 4; }
                default -> b.append(e); // \" \\ \/
            }
        }
    }
}
