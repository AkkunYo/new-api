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

import React, { useState } from 'react';
import { Card, Input, Button, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { IconSearch } from '@douyinfe/semi-icons';

const { Text } = Typography;

const TokenQueryForm = ({ onQuery, loading }) => {
  const { t } = useTranslation();
  const [token, setToken] = useState('');
  const [error, setError] = useState('');

  const validateToken = (value) => {
    const tokenRegex = /^sk-[a-zA-Z0-9]{48}$/;
    if (!value) {
      setError(t('请输入令牌'));
      return false;
    }
    if (!tokenRegex.test(value)) {
      setError(t('令牌格式不正确'));
      return false;
    }
    setError('');
    return true;
  };

  const handleSubmit = () => {
    if (validateToken(token)) {
      onQuery(token);
    }
  };

  const handleTokenChange = (value) => {
    setToken(value);
    if (error) {
      setError('');
    }
  };

  const handleKeyPress = (e) => {
    if (e.key === 'Enter') {
      handleSubmit();
    }
  };

  return (
    <Card style={{ marginBottom: 24 }}>
      <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
        <div style={{ flex: 1 }}>
          <Input
            size='large'
            prefix={<IconSearch />}
            placeholder={t('请输入令牌')}
            value={token}
            onChange={handleTokenChange}
            onKeyPress={handleKeyPress}
            disabled={loading}
            validateStatus={error ? 'error' : 'default'}
            style={{ width: '100%' }}
          />
          {error ? (
            <Text
              type='danger'
              size='small'
              style={{ marginTop: 8, display: 'block' }}
            >
              {error}
            </Text>
          ) : (
            <Text
              type='tertiary'
              size='small'
              style={{ marginTop: 8, display: 'block' }}
            >
              {t('令牌格式：sk- 开头的 48 字符')}
            </Text>
          )}
        </div>
        <Button
          theme='solid'
          size='large'
          onClick={handleSubmit}
          loading={loading}
          disabled={!token}
          style={{ minWidth: 100 }}
        >
          {t('查询')}
        </Button>
      </div>
    </Card>
  );
};

export default TokenQueryForm;
