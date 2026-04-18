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

import { useState, useCallback, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import axios from 'axios';
import {
  showError,
  showSuccess,
  timestamp2string,
  renderQuota,
  renderNumber,
  getLogOther,
  copy,
  renderClaudeLogContent,
  renderLogContent,
  renderAudioModelPrice,
  renderClaudeModelPrice,
  renderModelPrice,
  renderTaskBillingProcess,
} from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';

export const useTokenQueryData = () => {
  const { t } = useTranslation();

  // Define column keys
  const COLUMN_KEYS = {
    TIME: 'time',
    TOKEN: 'token',
    GROUP: 'group',
    TYPE: 'type',
    MODEL: 'model',
    USE_TIME: 'use_time',
    PROMPT: 'prompt',
    COMPLETION: 'completion',
    COST: 'cost',
    IP: 'ip',
    DETAILS: 'details',
  };

  // State
  const [token, setToken] = useState('');
  const [tokenUsage, setTokenUsage] = useState(null);
  const [logs, setLogs] = useState([]);
  const [expandData, setExpandData] = useState({});
  const [loading, setLoading] = useState(false);
  const [loadingUsage, setLoadingUsage] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [logCount, setLogCount] = useState(0);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);

  // Fixed column visibility for public query page
  const visibleColumns = useMemo(
    () => ({
      [COLUMN_KEYS.TIME]: true,
      [COLUMN_KEYS.TOKEN]: true,
      [COLUMN_KEYS.GROUP]: true,
      [COLUMN_KEYS.TYPE]: true,
      [COLUMN_KEYS.MODEL]: true,
      [COLUMN_KEYS.USE_TIME]: true,
      [COLUMN_KEYS.PROMPT]: true,
      [COLUMN_KEYS.COMPLETION]: true,
      [COLUMN_KEYS.COST]: true,
      [COLUMN_KEYS.IP]: true,
      [COLUMN_KEYS.DETAILS]: true,
    }),
    [COLUMN_KEYS],
  );

  // Billing display mode (default to price)
  const [billingDisplayMode, setBillingDisplayMode] = useState('price');

  // Create axios instance with token
  const createTokenAPI = useCallback((tokenValue) => {
    return axios.create({
      baseURL: import.meta.env.VITE_REACT_APP_SERVER_URL || '',
      headers: {
        Authorization: `Bearer ${tokenValue}`,
        'Cache-Control': 'no-store',
      },
    });
  }, []);

  // Validate token format
  const validateToken = useCallback((tokenValue) => {
    const tokenRegex = /^sk-[a-zA-Z0-9]{48}$/;
    return tokenRegex.test(tokenValue);
  }, []);

  // Fetch token usage
  const fetchTokenUsage = useCallback(
    async (tokenValue) => {
      if (!tokenValue) {
        showError(t('请先输入令牌'));
        return false;
      }

      if (!validateToken(tokenValue)) {
        showError(t('令牌格式不正确'));
        return false;
      }

      setLoadingUsage(true);
      try {
        const tokenAPI = createTokenAPI(tokenValue);
        const response = await tokenAPI.get('/api/usage/token/');
        if (response.data.code) {
          setTokenUsage(response.data.data);
          return true;
        } else {
          showError(response.data.message || t('查询失败'));
          return false;
        }
      } catch (error) {
        showError(
          error.response?.data?.message || error.message || t('查询失败'),
        );
        return false;
      } finally {
        setLoadingUsage(false);
      }
    },
    [t, validateToken, createTokenAPI],
  );

  // Fetch logs
  const fetchLogs = useCallback(
    async (tokenValue) => {
      if (!tokenValue) {
        showError(t('请先输入令牌'));
        return;
      }

      if (!validateToken(tokenValue)) {
        showError(t('令牌格式不正确'));
        return;
      }

      setLoading(true);
      try {
        const tokenAPI = createTokenAPI(tokenValue);
        const response = await tokenAPI.get('/api/log/token');

        if (response.data.success) {
          const logsData = response.data.data || [];

          // Process logs data
          const processedLogs = logsData.map((log, index) => {
            const logOther = getLogOther(log.other);

            return {
              key: `${log.id || index}`,
              time: timestamp2string(log.created_at),
              token_name: log.token_name,
              model_name: log.model_name,
              type: log.type,
              use_time: log.use_time,
              is_stream: log.is_stream,
              prompt_tokens: log.prompt_tokens,
              completion_tokens: log.completion_tokens,
              quota: log.quota,
              content: log.content,
              other: logOther,
              group: log.group,
              ip: log.ip,
              request_id: log.request_id,
              raw: log,
            };
          });

          setLogs(processedLogs);
          setLogCount(processedLogs.length);

          // Build expand data
          const newExpandData = {};
          processedLogs.forEach((log) => {
            const data = [];

            if (log.request_id) {
              data.push({ key: t('请求 ID'), value: log.request_id });
            }

            if (log.content) {
              let content = log.content;
              if (log.model_name?.includes('claude')) {
                content = renderClaudeLogContent(log.content);
              } else {
                content = renderLogContent(log.content);
              }
              data.push({ key: t('内容'), value: content });
            }

            newExpandData[log.key] = data;
          });

          setExpandData(newExpandData);
        } else {
          showError(response.data.message || t('查询失败'));
        }
      } catch (error) {
        showError(
          error.response?.data?.message || error.message || t('查询失败'),
        );
      } finally {
        setLoading(false);
      }
    },
    [t, validateToken, createTokenAPI],
  );

  // Query handler (fetch both usage and logs)
  const handleQuery = useCallback(
    async (tokenValue) => {
      const usageSuccess = await fetchTokenUsage(tokenValue);
      if (usageSuccess) {
        await fetchLogs(tokenValue);
      }
    },
    [fetchTokenUsage, fetchLogs],
  );

  // Page change handler
  const handlePageChange = useCallback((page) => {
    setActivePage(page);
  }, []);

  // Page size change handler
  const handlePageSizeChange = useCallback((size) => {
    setPageSize(size);
    setActivePage(1);
  }, []);

  // Copy text handler
  const copyText = useCallback(
    (text) => {
      copy(text, t);
    },
    [t],
  );

  // Check if has expandable rows
  const hasExpandableRows = useCallback(() => {
    return Object.values(expandData).some((data) => data && data.length > 0);
  }, [expandData]);

  return {
    // State
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

    // Constants
    COLUMN_KEYS,

    // Handlers
    handleQuery,
    handlePageChange,
    handlePageSizeChange,
    copyText,
    hasExpandableRows,
    validateToken,

    // Utils
    t,

    // Fixed values for table
    compactMode: false,
    isAdminUser: false,
    showUserInfoFunc: null,
    openChannelAffinityUsageCacheModal: null,
  };
};
