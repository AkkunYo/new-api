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
import { Typography, Card } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { useTokenQueryData } from '../../hooks/token-query/useTokenQueryData';
import TokenQueryForm from '../../components/token-query/TokenQueryForm';
import TokenUsageCard from '../../components/token-query/TokenUsageCard';
import UsageLogsTable from '../../components/table/usage-logs/UsageLogsTable';

const { Title, Text } = Typography;

const TokenQuery = () => {
  const { t } = useTranslation();
  const {
    token,
    setToken,
    tokenUsage,
    logs,
    expandData,
    loading,
    loadingUsage,
    activePage,
    logCount,
    pageSize,
    visibleColumns,
    billingDisplayMode,
    COLUMN_KEYS,
    handleQuery,
    handlePageChange,
    handlePageSizeChange,
    copyText,
    hasExpandableRows,
    compactMode,
    isAdminUser,
    showUserInfoFunc,
    openChannelAffinityUsageCacheModal,
  } = useTokenQueryData();

  return (
    <div
      style={{
        minHeight: 'calc(100vh - 64px)',
        padding: '40px 24px',
        maxWidth: 1200,
        margin: '0 auto',
        width: '100%',
      }}
    >
      <div style={{ marginBottom: 40, textAlign: 'center' }}>
        <Title heading={2} style={{ marginBottom: 8 }}>
          {t('令牌查询')}
        </Title>
        <Text type='tertiary' size='normal'>
          {t('输入您的 API Token 查看余额和使用记录')}
        </Text>
      </div>

      <TokenQueryForm
        onQuery={(tokenValue) => {
          setToken(tokenValue);
          handleQuery(tokenValue);
        }}
        loading={loadingUsage || loading}
      />

      <TokenUsageCard tokenUsage={tokenUsage} loading={loadingUsage} />

      {tokenUsage && (
        <Card
          title={t('调用记录')}
          style={{ marginBottom: 24 }}
          bodyStyle={{ padding: 0 }}
        >
          <UsageLogsTable
            logs={logs}
            expandData={expandData}
            loading={loading}
            activePage={activePage}
            pageSize={pageSize}
            logCount={logCount}
            compactMode={compactMode}
            visibleColumns={visibleColumns}
            handlePageChange={handlePageChange}
            handlePageSizeChange={handlePageSizeChange}
            copyText={copyText}
            showUserInfoFunc={showUserInfoFunc}
            openChannelAffinityUsageCacheModal={
              openChannelAffinityUsageCacheModal
            }
            hasExpandableRows={hasExpandableRows}
            isAdminUser={isAdminUser}
            billingDisplayMode='price'
            t={t}
            COLUMN_KEYS={COLUMN_KEYS}
          />
        </Card>
      )}
    </div>
  );
};

export default TokenQuery;
