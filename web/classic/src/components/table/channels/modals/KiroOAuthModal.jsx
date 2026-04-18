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
  Banner,
  Select,
  Spin,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../../helpers';

const { Text } = Typography;

const KiroOAuthModal = ({ visible, onCancel, onSuccess }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [region, setRegion] = useState('us-east-1');

  // Builder ID 状态
  const [taskId, setTaskId] = useState('');
  const [userCode, setUserCode] = useState('');
  const [verificationUri, setVerificationUri] = useState('');
  const [verificationUriComplete, setVerificationUriComplete] = useState('');
  const [polling, setPolling] = useState(false);
  const [pollInterval, setPollInterval] = useState(null);

  const startBuilderIDOAuth = async () => {
    setLoading(true);
    try {
      const res = await API.post(
        '/api/channel/kiro/oauth/builderid/start',
        { region },
        { skipErrorHandler: true },
      );
      if (!res?.data?.success) {
        throw new Error(res?.data?.message || t('启动授权失败'));
      }

      const data = res?.data?.data || {};
      setTaskId(data.task_id || '');
      setUserCode(data.user_code || '');
      setVerificationUri(data.verification_uri || '');
      setVerificationUriComplete(data.verification_uri_complete || '');

      // 自动打开授权页面（使用完整 URL，包含 user_code）
      if (data.verification_uri_complete) {
        window.open(
          data.verification_uri_complete,
          '_blank',
          'noopener,noreferrer',
        );
      } else if (data.verification_uri) {
        window.open(data.verification_uri, '_blank', 'noopener,noreferrer');
      }

      showSuccess(t('已打开授权页面，后端正在自动轮询...'));

      // 开始轮询
      startPolling(data.task_id);
    } catch (error) {
      showError(error?.message || t('启动授权失败'));
    } finally {
      setLoading(false);
    }
  };

  const startPolling = (tid) => {
    if (!tid) {
      showError(t('缺少任务 ID'));
      return;
    }

    setPolling(true);
    const interval = setInterval(async () => {
      try {
        const res = await API.post(
          '/api/channel/kiro/oauth/builderid/poll',
          { task_id: tid },
          { skipErrorHandler: true },
        );

        if (res?.data?.success) {
          // 授权成功
          clearInterval(interval);
          setPolling(false);

          const key = res?.data?.data?.key || '';
          if (!key) {
            throw new Error(t('响应缺少凭据'));
          }

          onSuccess && onSuccess(key);
          showSuccess(t('授权成功'));
          onCancel && onCancel();
        } else if (res?.data?.data?.status === 'pending') {
          // 继续等待
        } else if (res?.data?.data?.status === 'slow_down') {
          // 减慢轮询速度
        } else {
          // 其他错误
          clearInterval(interval);
          setPolling(false);
          showError(res?.data?.message || t('授权失败'));
        }
      } catch (error) {
        clearInterval(interval);
        setPolling(false);
        showError(error?.message || t('轮询失败'));
      }
    }, 3000);

    setPollInterval(interval);
  };

  useEffect(() => {
    return () => {
      if (pollInterval) {
        clearInterval(pollInterval);
      }
    };
  }, [pollInterval]);

  const handleCancel = () => {
    if (pollInterval) {
      clearInterval(pollInterval);
      setPollInterval(null);
    }
    setPolling(false);
    setTaskId('');
    setUserCode('');
    setVerificationUri('');
    setVerificationUriComplete('');
    onCancel && onCancel();
  };

  return (
    <Modal
      title={t('Kiro 授权')}
      visible={visible}
      onCancel={handleCancel}
      maskClosable={false}
      closeOnEsc
      width={600}
      footer={
        <Space>
          <Button
            theme='borderless'
            onClick={handleCancel}
            disabled={loading || polling}
          >
            {t('取消')}
          </Button>
        </Space>
      }
    >
      <Space vertical spacing='tight' style={{ width: '100%' }}>
        <Banner
          type='info'
          description={t(
            '使用 AWS Builder ID 进行授权。授权后将自动生成凭据。',
          )}
        />

        <Space vertical align='start' style={{ width: '100%' }}>
          <Text strong>{t('选择区域')}</Text>
          <Select
            value={region}
            onChange={setRegion}
            style={{ width: '100%' }}
            disabled={polling || loading}
          >
            <Select.Option value='us-east-1'>us-east-1</Select.Option>
            <Select.Option value='us-west-2'>us-west-2</Select.Option>
            <Select.Option value='eu-west-1'>eu-west-1</Select.Option>
            <Select.Option value='ap-southeast-1'>ap-southeast-1</Select.Option>
          </Select>
        </Space>

        {!taskId && (
          <Button
            type='primary'
            onClick={startBuilderIDOAuth}
            loading={loading}
            block
          >
            {t('启动 Builder ID 授权')}
          </Button>
        )}

        {taskId && (
          <Space vertical align='start' style={{ width: '100%' }}>
            <Banner
              type='success'
              description={
                <Space vertical align='start'>
                  <Text>{t('授权流程已启动，请在打开的页面中完成授权')}</Text>
                  {userCode && (
                    <Text>
                      {t('用户码')}: <Text strong>{userCode}</Text>
                    </Text>
                  )}
                  {verificationUri && (
                    <Text>
                      {t('授权地址')}: <Text code>{verificationUri}</Text>
                    </Text>
                  )}
                </Space>
              }
            />

            {polling && (
              <Space>
                <Spin />
                <Text>{t('等待授权完成...')}</Text>
              </Space>
            )}
          </Space>
        )}

        <Text type='tertiary' size='small'>
          {t(
            '说明：授权成功后将自动生成包含 access_token 和 refresh_token 的凭据。',
          )}
        </Text>
      </Space>
    </Modal>
  );
};

export default KiroOAuthModal;
