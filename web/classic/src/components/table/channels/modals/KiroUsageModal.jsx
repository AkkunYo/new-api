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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Modal,
  Button,
  Progress,
  Typography,
  Spin,
  Tag,
  Descriptions,
  Collapse,
} from '@douyinfe/semi-ui';
import { API, showError } from '../../../../helpers';

const { Text } = Typography;

const clampPercent = (value) => {
  const v = Number(value);
  if (!Number.isFinite(v)) return 0;
  return Math.max(0, Math.min(100, v));
};

const pickStrokeColor = (percent) => {
  const p = clampPercent(percent);
  if (p >= 95) return '#ef4444';
  if (p >= 80) return '#f59e0b';
  return '#3b82f6';
};

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

const KiroUsageView = ({ t, record, payload, onCopy, onRefresh }) => {
  const tt = typeof t === 'function' ? t : (v) => v;
  const [showRawJson, setShowRawJson] = useState(false);
  const data = payload?.data ?? null;
  const upstreamStatus = payload?.upstream_status;

  // 从 usageBreakdownList 中获取第一个资源的用量信息
  const breakdown = data?.usageBreakdownList?.[0] ?? null;
  const usedCount = Number(
    breakdown?.currentUsageWithPrecision ?? breakdown?.currentUsage ?? 0,
  );
  const limitCount = Number(
    breakdown?.usageLimitWithPrecision ?? breakdown?.usageLimit ?? 0,
  );
  const email = data?.userInfo?.email;
  const resourceType = breakdown?.resourceType;
  const displayName = breakdown?.displayName;

  const percent =
    limitCount > 0 ? Math.round(clampPercent((usedCount / limitCount) * 100) * 10) / 10 : 0;
  const remaining = limitCount > 0 ? limitCount - usedCount : 0;

  const errorMessage =
    payload?.success === false
      ? getDisplayText(payload?.message) || tt('获取用量失败')
      : '';

  const rawText =
    typeof data === 'string' ? data : JSON.stringify(data ?? payload, null, 2);

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
              {tt('Kiro 帐号')}
            </div>
            <div className='mt-2 flex flex-wrap items-center gap-2'>
              <Tag
                color='blue'
                type='light'
                shape='circle'
                size='large'
                className='font-semibold'
              >
                Kiro
              </Tag>
              {statusTag}
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
            <Descriptions.Item itemKey={tt('邮箱')}>
              <AccountInfoValue t={tt} value={email} onCopy={onCopy} />
            </Descriptions.Item>
            <Descriptions.Item itemKey={tt('用户ID')}>
              <AccountInfoValue
                t={tt}
                value={data?.userInfo?.userId}
                onCopy={onCopy}
                monospace={true}
              />
            </Descriptions.Item>
            <Descriptions.Item itemKey={tt('订阅类型')}>
              <AccountInfoValue
                t={tt}
                value={data?.subscriptionInfo?.subscriptionTitle}
                onCopy={onCopy}
              />
            </Descriptions.Item>
            <Descriptions.Item itemKey={tt('资源类型')}>
              <AccountInfoValue
                t={tt}
                value={displayName || resourceType}
                onCopy={onCopy}
              />
            </Descriptions.Item>
          </Descriptions>
        </div>

        <div className='mt-2 text-xs text-semi-color-text-2'>
          {tt('渠道：')}
          {record?.name || '-'} ({tt('编号：')}
          {record?.id || '-'})
        </div>
      </div>

      <div>
        <div className='mb-2'>
          <div className='text-sm font-semibold text-semi-color-text-0'>
            {tt('配额使用情况')}
          </div>
          <Text type='tertiary' size='small'>
            {tt('用于观察当前帐号在 Kiro 上游的配额使用情况')}
          </Text>
        </div>
      </div>

      <div className='rounded-lg border border-semi-color-border bg-semi-color-bg-0 p-3'>
        <div className='mb-2 flex items-center justify-between gap-2'>
          <div className='font-medium'>{tt('配额使用')}</div>
          <Text type='tertiary' size='small'>
            {usedCount} / {limitCount}
          </Text>
        </div>

        <div className='mt-2'>
          <Progress
            percent={percent}
            stroke={pickStrokeColor(percent)}
            showInfo={true}
          />
        </div>

        <div className='mt-2 flex flex-wrap items-center gap-4 text-xs text-semi-color-text-2'>
          <div>
            {tt('已使用：')}
            <span className='ml-1 font-medium text-semi-color-text-0'>
              {usedCount}
            </span>
          </div>
          <div>
            {tt('总配额：')}
            <span className='ml-1 font-medium text-semi-color-text-0'>
              {limitCount}
            </span>
          </div>
          <div>
            {tt('剩余：')}
            <span className='ml-1 font-medium text-semi-color-text-0'>
              {remaining}
            </span>
          </div>
          <div>
            {tt('使用率：')}
            <span className='ml-1 font-medium text-semi-color-text-0'>
              {percent.toFixed(1)}%
            </span>
          </div>
        </div>
      </div>

      {breakdown?.freeTrialInfo?.freeTrialStatus === 'ACTIVE' && (
        <div className='rounded-lg border border-green-200 bg-green-50 p-3'>
          <div className='mb-2 flex items-center gap-2'>
            <span className='text-base'>🎁</span>
            <div className='font-medium text-green-800'>{tt('免费试用')}</div>
          </div>

          <div className='flex flex-wrap items-center gap-4 text-xs text-green-700'>
            <div>
              {tt('已使用：')}
              <span className='ml-1 font-medium text-green-800'>
                {breakdown.freeTrialInfo.currentUsageWithPrecision ??
                  breakdown.freeTrialInfo.currentUsage}
              </span>
            </div>
            <div>
              {tt('总配额：')}
              <span className='ml-1 font-medium text-green-800'>
                {breakdown.freeTrialInfo.usageLimitWithPrecision ??
                  breakdown.freeTrialInfo.usageLimit}
              </span>
            </div>
            <div>
              {tt('到期时间：')}
              <span className='ml-1 font-medium text-green-800'>
                {breakdown.freeTrialInfo.freeTrialExpiry
                  ? new Date(
                      breakdown.freeTrialInfo.freeTrialExpiry * 1000,
                    ).toLocaleString()
                  : '-'}
              </span>
            </div>
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

const KiroUsageLoader = ({ t, record, initialPayload, onCopy }) => {
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
      const res = await API.get(`/api/channel/${recordId}/kiro/usage`, {
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
    <KiroUsageView
      t={tt}
      record={record}
      payload={payload}
      onCopy={onCopy}
      onRefresh={fetchUsage}
    />
  );
};

export const openKiroUsageModal = ({ t, record, payload, onCopy }) => {
  const tt = typeof t === 'function' ? t : (v) => v;

  Modal.info({
    title: tt('Kiro 帐号与用量'),
    centered: true,
    width: 900,
    style: { maxWidth: '95vw' },
    content: (
      <KiroUsageLoader
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
