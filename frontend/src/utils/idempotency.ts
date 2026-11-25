// 幂等键：一次用户动作一个键。写请求（POST）默认携带，重复提交时后端直接回放首响应。

/** ULID 字符表（Crockford Base32，去掉易混的 I/L/O/U） */
const ULID_ALPHABET = '0123456789ABCDEFGHJKMNPQRSTVWXYZ'

/** 生成 26 位 ULID（时间有序，与后端 ID 风格一致；仅用于客户端请求标识） */
export function newActionId(): string {
  let timePart = Date.now()
  let id = ''
  for (let index = 0; index < 10; index += 1) {
    id = ULID_ALPHABET[timePart % 32] + id
    timePart = Math.floor(timePart / 32)
  }
  const randomBytes = new Uint8Array(16)
  if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
    crypto.getRandomValues(randomBytes)
  } else {
    for (let index = 0; index < randomBytes.length; index += 1) {
      randomBytes[index] = Math.floor(Math.random() * 256)
    }
  }
  for (let index = 0; index < 16; index += 1) {
    id += ULID_ALPHABET[randomBytes[index] % 32]
  }
  return id
}
