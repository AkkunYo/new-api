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

import React from 'react';
import {
  Card,
  Descriptions,
  Tag,
  Spin,
  Space,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { renderQuota, timestamp2string } from '../../helpers';

const { Text } = Typography;

const TokenUsageCard = ({ tokenUsage, loading }) => {
  const { t } = useTranslation();

  if (loading) {
    return (
      <Card style={{ marginBottom: 24 }}>
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin size='large' />
        </div>
      </Card>
    );
  }

  if (!tokenUsage) {
    return null;
  }

  const data = [
    {
      key: t('令牌名称'),
      value: tokenUsage.name || '-',
    },
    {
      key: t('总额度'),
      value: tokenUsage.unlimited_quota ? (
        <Tag color='green' size='large'>
          {t('无限额度')}
        </Tag>
      ) : (
        <Text strong>{renderQuota(tokenUsage.total_granted, 2)}</Text>
      ),
    },
    {
      key: t('已用额度'),
      value: tokenUsage.unlimited_quota ? (
        '-'
      ) : (
        <Text type='tertiary'>{renderQuota(tokenUsage.total_used, 2)}</Text>
      ),
    },
    {
      key: t('剩余额度'),
      value: tokenUsage.unlimited_quota ? (
        <Tag color='green' size='large'>
          {t('无限额度')}
        </Tag>
      ) : (
        <Text strong style={{ color: 'var(--semi-color-success)' }}>
          {renderQuota(tokenUsage.total_available, 2)}
        </Text>
      ),
    },
    {
      key: t('过期时间'),
      value:
        tokenUsage.expires_at === -1 ? (
          <Tag color='blue'>{t('永不过期')}</Tag>
        ) : (
          timestamp2string(tokenUsage.expires_at)
        ),
    },
  ];

  // Add model limits if enabled
  if (tokenUsage.model_limits_enabled && tokenUsage.model_limits) {
    const modelLimitsArray = Object.keys(tokenUsage.model_limits);
    data.push({
      key: t('模型限制'),
      value: (
        <Space wrap>
          <Tag color='blue'>{t('已启用')}</Tag>
          {modelLimitsArray.length > 0 && (
            <Text type='tertiary' size='small'>
              ({modelLimitsArray.length} {t('个模型')})
            </Text>
          )}
        </Space>
      ),
    });
  } else {
    data.push({
      key: t('模型限制'),
      value: <Tag color='grey'>{t('未启用')}</Tag>,
    });
  }

  return (
    <Card
      title={
        <Text strong size='large'>
          {t('令牌信息')}
        </Text>
      }
      style={{ marginBottom: 24 }}
      headerStyle={{ borderBottom: '1px solid var(--semi-color-border)' }}
    >
      <Descriptions data={data} row size='medium' style={{ marginTop: 8 }} />
    </Card>
  );
};

export default TokenUsageCard;
