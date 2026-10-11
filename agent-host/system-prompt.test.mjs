import { expect, test } from 'bun:test';
import { SYSTEM_PROMPT } from './system-prompt.mjs';

test('media policy chooses minimum evidence and never presents frames as watched or heard media', () => {
  for (const instruction of ['最低必要', 'mode=frames', 'mode=video', 'mode=audio', '静态截图', '不能据此声称', '15 秒', '分段', '实际读取的区间']) {
    expect(SYSTEM_PROMPT).toContain(instruction);
  }
  expect(SYSTEM_PROMPT).toContain('不必读取原生视频或音频');
  expect(SYSTEM_PROMPT).toContain('用户明确要求看视频并听音频');
});
