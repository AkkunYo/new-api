/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MEESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Modal,
  Button,
  Typography,
  Spin,
  Tag,
  Descriptions,
  Collapse,
} from '@douyinfe/semi-ui';
import { API, showError } from '../../../../helpers';

const { Text } = Typography;

const getDisplayText = (value) => {
  if (value == null) return '';
  return String(value).trim();
};

const AccountInfoValue = ({ t, value, onCopy, monospace = false }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const text = getDisplayText(value);
  const hasValue = text !== '';

  return (
    <div className='flex min-w-0 items-start justify-between gap-2'>
      <div
        className={`min-w-0 flex-1 break-all text-xs leading-5 text-semi-color-text-1 ${
          monospace ? 'font-mono' : ''
        }`}
      >
        {hasValue ? text : '-'}
      </div>
      <Button
        size='small'
        type='tertiary'
        theme='borderless'
        className='shrink-0 px-1 text-xs'
        disabled={!hasValue}
        onClick={() => onCopy?.(text)}
      >
        {tt('复制')}
      </Button>
    </div>
  );
};

const MODEL_GROUPS = [
  { label: 'Gemini Pro', keys: ['gemini-2.5-pro', 'gemini-3-pro-low', 'gemini-3-pro-high', 'gemini-3-pro-preview', 'gemini-3.1-pro-low', 'gemini-3.1-pro-high', 'gemini-pro-agent'], color: '#6366f1' },
  { label: 'Gemini Flash', keys: ['gemini-2.5-flash', 'gemini-2.5-flash-lite', 'gemini-2.0-flash', 'gemini-3-flash', 'gemini-3-flash-agent', 'gemini-3.1-flash-lite', 'gemini-3.5-flash-low', 'gemini-3.5-flash-extra-low'], color: '#10b981' },
  { label: 'Gemini Image', keys: ['gemini-2.5-flash-image', 'gemini-3.1-flash-image', 'gemini-3-pro-image'], color: '#a855f7' },
  { label: 'Claude', keys: ['claude-sonnet-4-5', 'claude-sonnet-4-5-thinking', 'claude-sonnet-4-6', 'claude-opus-4-5-thinking', 'claude-opus-4-6', 'claude-opus-4-6-thinking', 'claude-opus-4-7', 'claude-sonnet-4', 'claude-sonnet-4-20250514'], color: '#f59e0b' },
];

function extractModelQuotaBars(modelsQuota) {
  const models = modelsQuota?.models;
  if (!models) return [];
  const bars = [];
  for (const group of MODEL_GROUPS) {
    let maxUtil = -1;
    let earliestReset = null;
    let hasModel = false;
    for (const key of group.keys) {
      const m = models[key];
      if (!m) continue;
      hasModel = true;
      if (!m.quotaInfo) continue;
      const util = Math.round((1 - (m.quotaInfo.remainingFraction ?? 1)) * 100);
      if (util > maxUtil) maxUtil = util;
      const rt = m.quotaInfo.resetTime ?? null;
      if (rt && (!earliestReset || rt < earliestReset)) earliestReset = rt;
    }
    if (!hasModel) continue;
    bars.push({ label: group.label, utilization: maxUtil < 0 ? null : maxUtil, resetTime: earliestReset, color: group.color });
  }
  return bars;
}

function formatResetTime(resetTime) {
  if (!resetTime) return '';
  const d = new Date(resetTime);
  if (isNaN(d.getTime())) return '';
  const diffMs = d.getTime() - Date.now();
  if (diffMs <= 0) return 'now';
  const diffMin = Math.floor(diffMs / 60000);
  const diffH = Math.floor(diffMin / 60);
  if (diffH >= 1) return `${diffH}h${diffMin % 60}m`;
  return `${diffMin}m`;
}

const AntigravityUsageView = ({ t, record, payload, onCopy, onRefresh }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const [showRawJson, setShowRawJson] = useState(false);
  const data = payload?.data ?? null;
  const upstreamStatus = payload?.upstream_status;

  // 从 loadCodeAssist 响应中提取字段
  const projectId =
    payload?.key_project_id ||
    data?.cloudaicompanionProject ||
    data?.cloudAiCompanionProject ||
    data?.projectId ||
    data?.project_id ||
    '';
  const currentTierId = data?.currentTier?.id || '';
  const paidTier = data?.paidTier ?? null;
  const membershipName = paidTier?.name || paidTier?.id || '';
  const g1Credit = paidTier?.availableCredits?.find(
    (c) => c?.creditType?.toUpperCase() === 'GOOGLE_ONE_AI'
  ) ?? null;
  const creditAmount = Number(g1Credit?.creditAmount ?? 0);
  const minAmount = Number(g1Credit?.minimumCreditAmountForUsage ?? 0);
  const creditsAvailable = g1Credit != null && creditAmount >= minAmount;
  const modelQuotaBars = extractModelQuotaBars(payload?.models_quota);

  const errorMessage =
    payload?.success === false
      ? getDisplayText(payload?.message) || tt('获取用量失败')
      : '';

  const rawText =
    typeof data === 'string' ? data : JSON.stringify(payload, null, 2);

  const statusTag = payload?.success ? (
    <Tag color='green'>{tt('可用')}</Tag>
  ) : (
    <Tag color='red'>{tt('获取失败')}</Tag>
  );

  return (
    <div className='flex flex-col gap-4'>
      {errorMessage && (
        <div className='rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700'>
          {errorMessage}
        </div>
      )}

      <div className='rounded-xl border border-semi-color-border bg-semi-color-bg-0 p-3'>
        <div className='flex flex-wrap items-start justify-between gap-2'>
          <div className='min-w-0'>
            <div className='text-xs font-medium text-semi-color-text-2'>
              {tt('Antigravity 帐号')}
            </div>
            <div className='mt-2 flex flex-wrap items-center gap-2'>
              <Tag
                color='blue'
                type='light'
                shape='circle'
                size='large'
                className='font-semibold'
              >
                Antigravity
              </Tag>
              {statusTag}
              {currentTierId && (
                <Tag color='purple' type='light' shape='circle'>
                  {currentTierId}
                </Tag>
              )}
              {membershipName && (
                <Tag color='green' type='light' shape='circle'>
                  {membershipName}
                </Tag>
              )}
              <Tag color='grey' type='light' shape='circle'>
                {tt('上游状态码：')}
                {upstreamStatus ?? '-'}
              </Tag>
            </div>
          </div>
          <Button
            size='small'
            type='tertiary'
            theme='outline'
            onClick={onRefresh}
          >
            {tt('刷新')}
          </Button>
        </div>

        <div className='mt-2 rounded-lg bg-semi-color-fill-0 px-3 py-2'>
            <Descriptions>
            <Descriptions.Item itemKey={tt('项目ID')}>
              <AccountInfoValue
                t={tt}
                value={projectId}
                onCopy={onCopy}
                monospace={true}
              />
            </Descriptions.Item>
            {membershipName && (
              <Descriptions.Item itemKey={tt('会员身份')}>
                <AccountInfoValue t={tt} value={membershipName} onCopy={onCopy} />
              </Descriptions.Item>
            )}
            {paidTier?.id && paidTier?.id !== membershipName && (
              <Descriptions.Item itemKey={tt('付费套餐ID')}>
                <AccountInfoValue t={tt} value={paidTier.id} onCopy={onCopy} monospace={true} />
              </Descriptions.Item>
            )}
            {currentTierId && (
              <Descriptions.Item itemKey={tt('代码助手档位')}>
                <AccountInfoValue t={tt} value={currentTierId} onCopy={onCopy} monospace={true} />
              </Descriptions.Item>
            )}
          </Descriptions>
        </div>

        <div className='mt-2 text-xs text-semi-color-text-2'>
          {tt('渠道：')}
          {record?.name || '-'} ({tt('编号：')}
          {record?.id || '-'})
        </div>
      </div>

      {g1Credit && (
        <div className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'>
          <div className='flex items-center justify-between gap-2'>
            <div className='font-medium'>{tt('Google One AI 额度')}</div>
            <Tag
              color={creditsAvailable ? 'green' : 'red'}
              type='light'
              shape='circle'
            >
              {creditsAvailable ? tt('可用') : tt('额度不足')}
            </Tag>
          </div>

          <div className='mt-2 flex flex-wrap items-center gap-4 text-xs text-semi-color-text-2'>
            <div>
              {tt('当前余量：')}
              <span className='ml-1 font-medium text-semi-color-text-0'>
                {g1Credit.creditAmount ?? '-'}
              </span>
            </div>
            <div>
              {tt('最低可用门槛：')}
              <span className='ml-1 font-medium text-semi-color-text-0'>
                {g1Credit.minimumCreditAmountForUsage ?? '-'}
              </span>
            </div>
          </div>
        </div>
      )}

      {modelQuotaBars.length > 0 && (
        <div className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'>
          <div className='mb-2 font-medium'>{tt('模型配额')}</div>
          <div className='flex flex-col gap-2'>
            {modelQuotaBars.map((bar) => {
              const pct = bar.utilization != null ? Math.min(100, Math.max(0, bar.utilization)) : null;
              const reset = formatResetTime(bar.resetTime);
              return (
                <div key={bar.label} className='flex items-center gap-2'>
                  <span className='w-24 flex-shrink-0 text-xs text-semi-color-text-2'>{bar.label}</span>
                  <div className='h-1.5 flex-1 overflow-hidden rounded-full bg-semi-color-fill-1'>
                    {pct != null && (
                      <div
                        style={{ width: `${pct}%`, backgroundColor: bar.color, height: '100%', borderRadius: '9999px', transition: 'width 0.3s' }}
                      />
                    )}
                  </div>
                  <span className='w-9 flex-shrink-0 text-right text-xs font-medium'>
                    {pct != null ? `${pct}%` : 'N/A'}
                  </span>
                  {reset && (
                    <span className='flex-shrink-0 text-xs text-semi-color-text-2'>{reset}</span>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      <Collapse
        activeKey={showRawJson ? ['raw-json'] : []}
        onChange={(activeKey) => {
          const keys = Array.isArray(activeKey) ? activeKey : [activeKey];
          setShowRawJson(keys.includes('raw-json'));
        }}
      >
        <Collapse.Panel header={tt('原始 JSON')} itemKey='raw-json'>
          <div className='mb-2 flex justify-end'>
            <Button
              size='small'
              type='primary'
              theme='outline'
              onClick={() => onCopy?.(rawText)}
              disabled={!rawText}
            >
              {tt('复制')}
            </Button>
          </div>
          <pre className='max-h-[50vh] overflow-y-auto rounded-lg bg-semi-color-fill-0 p-3 text-xs text-semi-color-text-0'>
            {rawText}
          </pre>
        </Collapse.Panel>
      </Collapse>
    </div>
  );
};

const AntigravityUsageLoader = ({ t, record, initialPayload, onCopy }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const [loading, setLoading] = useState(!initialPayload);
  const [payload, setPayload] = useState(initialPayload ?? null);
  const hasShownErrorRef = useRef(false);
  const mountedRef = useRef(true);
  const recordId = record?.id;

  const fetchUsage = useCallback(async () => {
    if (!recordId) {
      if (mountedRef.current) setPayload(null);
      return;
    }

    if (mountedRef.current) setLoading(true);
    hasShownErrorRef.current = false;
    try {
      const res = await API.get(`/api/channel/${recordId}/antigravity/usage`, {
        skipErrorHandler: true,
      });
      if (!mountedRef.current) return;
      setPayload(res?.data ?? null);
      if (!res?.data?.success && !hasShownErrorRef.current) {
        hasShownErrorRef.current = true;
        showError(tt('获取用量失败'));
      }
    } catch (error) {
      if (!mountedRef.current) return;
      if (!hasShownErrorRef.current) {
        hasShownErrorRef.current = true;
        showError(tt('获取用量失败'));
      }
      setPayload({ success: false, message: String(error) });
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, [recordId, tt]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (initialPayload) return;
    fetchUsage().catch(() => {});
  }, [fetchUsage, initialPayload]);

  if (loading) {
    return (
      <div className='flex items-center justify-center py-10'>
        <Spin spinning={true} size='large' tip={tt('加载中...')} />
      </div>
    );
  }

  if (!payload) {
    return (
      <div className='flex flex-col gap-3'>
        <Text type='danger'>{tt('获取用量失败')}</Text>
        <div className='flex justify-end'>
          <Button
            size='small'
            type='primary'
            theme='outline'
            onClick={fetchUsage}
          >
            {tt('刷新')}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <AntigravityUsageView
      t={tt}
      record={record}
      payload={payload}
      onCopy={onCopy}
      onRefresh={fetchUsage}
    />
  );
};

export const openAntigravityUsageModal = ({ t, record, payload, onCopy }) => {
  const tt = typeof t === 'function' ? t : (v) => v;

  Modal.info({    title: tt('Antigravity 帐号与用量'),
    centered: true,
    width: 900,
    style: { maxWidth: '95vw' },
    content: (
      <AntigravityUsageLoader
        t={tt}
        record={record}
        initialPayload={payload}
        onCopy={onCopy}
      />
    ),
    footer: (
      <div className='flex justify-end gap-2'>
        <Button type='primary' theme='solid' onClick={() => Modal.destroyAll()}>
          {tt('关闭')}
        </Button>
      </div>
    ),
  });
};
