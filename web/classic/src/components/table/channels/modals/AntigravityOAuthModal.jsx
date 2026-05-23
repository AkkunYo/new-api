/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Modal,
  Button,
  Space,
  Typography,
  Input,
  Banner,
} from '@douyinfe/semi-ui';
import { API, copy, showError, showSuccess } from '../../../../helpers';

const { Text } = Typography;

/**
 * Antigravity Google OAuth 授权 Modal
 *
 * 流程（同 Codex）：
 *   1. 点「打开授权页面」→ 后端生成 authorize_url + 存 session state → 浏览器跳 Google OAuth
 *   2. 用户完成登录，浏览器重定向到 localhost（页面打不开没关系）
 *   3. 复制地址栏完整回调 URL 粘贴到输入框
 *   4. 点「生成并填入」→ 后端解析 code + state，交换 token，发现 project_id → 返回 JSON key
 */
const AntigravityOAuthModal = ({ visible, onCancel, onSuccess, channelId }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [authorizeUrl, setAuthorizeUrl] = useState('');
  const [input, setInput] = useState('');

  useEffect(() => {
    if (!visible) return;
    setAuthorizeUrl('');
    setInput('');
  }, [visible]);

  const startOAuth = async () => {
    setLoading(true);
    try {
      const endpoint = channelId
        ? `/api/channel/${channelId}/antigravity/oauth/start`
        : '/api/channel/antigravity/oauth/start';
      const res = await API.post(endpoint, {}, { skipErrorHandler: true });
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || t('启动授权失败'));
      }
      const authUrl = res?.data?.data?.authorize_url || '';
      if (!authUrl) {
        throw new Error(t('响应缺少授权链接'));
      }
      setAuthorizeUrl(authUrl);
      window.open(authUrl, '_blank', 'noopener,noreferrer');
      showSuccess(t('已打开授权页面'));
    } catch (error) {
      showError(error?.message || t('启动授权失败'));
    } finally {
      setLoading(false);
    }
  };

  const completeOAuth = async () => {
    const v = (input || '').trim();
    if (!v) {
      showError(t('请先粘贴回调 URL'));
      return;
    }

    setLoading(true);
    try {
      const endpoint = channelId
        ? `/api/channel/${channelId}/antigravity/oauth/complete`
        : '/api/channel/antigravity/oauth/complete';
      const res = await API.post(
        endpoint,
        { input: v },
        { skipErrorHandler: true },
      );
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || t('授权失败'));
      }

      if (channelId) {
        // 已有渠道：后端已自动写入，直接提示
        const { project_id, expires_at } = res.data.data || {};
        showSuccess(
          project_id
            ? t('授权成功，项目 ID: {{id}}', { id: project_id })
            : t('授权成功'),
        );
        onCancel && onCancel();
        return;
      }

      // 新建渠道：拿到 key 回填
      const key = res?.data?.data?.key || '';
      if (!key) {
        throw new Error(t('响应缺少凭据'));
      }
      onSuccess && onSuccess(key);
      showSuccess(t('已生成授权凭据'));
      onCancel && onCancel();
    } catch (error) {
      showError(error?.message || t('授权失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title={t('Antigravity Google 授权')}
      visible={visible}
      onCancel={onCancel}
      maskClosable={false}
      closeOnEsc
      width={720}
      footer={
        <Space>
          <Button theme='borderless' onClick={onCancel} disabled={loading}>
            {t('取消')}
          </Button>
          <Button
            theme='solid'
            type='primary'
            onClick={completeOAuth}
            loading={loading}
            disabled={!authorizeUrl}
          >
            {channelId ? t('完成授权') : t('生成并填入')}
          </Button>
        </Space>
      }
    >
      <Space vertical spacing='tight' style={{ width: '100%' }}>
        <Banner
          type='info'
          description={t(
            '1) 点击「打开授权页面」完成 Google 登录；2) 浏览器会跳转到 localhost（页面打不开也没关系）；3) 复制地址栏完整 URL 粘贴到下方；4) 点击「完成授权」或「生成并填入」。',
          )}
        />

        <Space wrap>
          <Button type='primary' onClick={startOAuth} loading={loading}>
            {t('打开授权页面')}
          </Button>
          <Button
            theme='outline'
            disabled={!authorizeUrl || loading}
            onClick={() => copy(authorizeUrl)}
          >
            {t('复制授权链接')}
          </Button>
        </Space>

        <Input
          value={input}
          onChange={(value) => setInput(value)}
          placeholder={t('请粘贴完整回调 URL（包含 code 与 state 参数）')}
          showClear
          disabled={!authorizeUrl || loading}
        />

        <Text type='tertiary' size='small'>
          {t(
            '说明：授权成功后系统会自动发现并保存 Google Cloud project_id，后续 access_token 会在临期前 50 分钟自动刷新。',
          )}
        </Text>
      </Space>
    </Modal>
  );
};

export default AntigravityOAuthModal;
