import React, { useEffect, useState, useRef } from 'react';
import {
  Banner,
  Button,
  Row,
  Col,
  Switch,
  Input,
  InputNumber,
  Typography,
  Space,
  Popconfirm,
  Card,
  Empty,
} from '@douyinfe/semi-ui';
import {
  IconPlus,
  IconDelete,
  IconArrowUp,
  IconArrowDown,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, showWarning } from '../../../helpers';

const { Text } = Typography;

export default function SettingsUAMatch(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [rules, setRules] = useState([]);
  // 保存一份原始快照，用于判断是否有变更
  const snapRef = useRef({ enabled: false, rulesJson: '[]' });

  useEffect(() => {
    if (!props.options) return;

    const rawEnabled = props.options['ua_match_setting.enabled'];
    const enabledVal =
      rawEnabled === 'true' || rawEnabled === true;
    setEnabled(enabledVal);

    const rawRules = props.options['ua_match_setting.rules'];
    if (rawRules) {
      let parsed;
      try {
        parsed =
          typeof rawRules === 'string' ? JSON.parse(rawRules) : rawRules;
      } catch {
        parsed = [];
      }
      if (Array.isArray(parsed)) {
        setRules(parsed);
        snapRef.current = {
          enabled: enabledVal,
          rulesJson: JSON.stringify(parsed),
        };
      }
    }
  }, [props.options]);

  // ---- 规则操作 ----

  function addRule() {
    setRules((prev) => [
      ...prev,
      { name: '', regex: '', status_code: 200, body: '' },
    ]);
  }

  function removeRule(index) {
    setRules((prev) => prev.filter((_, i) => i !== index));
  }

  function moveRule(index, direction) {
    setRules((prev) => {
      const arr = [...prev];
      const target = index + direction;
      if (target < 0 || target >= arr.length) return prev;
      [arr[index], arr[target]] = [arr[target], arr[index]];
      return arr;
    });
  }

  function updateRule(index, field, value) {
    setRules((prev) => {
      const arr = [...prev];
      arr[index] = { ...arr[index], [field]: value };
      return arr;
    });
  }

  // ---- 保存 ----

  async function onSubmit() {
    // 校验
    for (let i = 0; i < rules.length; i++) {
      const r = rules[i];
      if (!r.regex.trim()) {
        showError(t('第 {{index}} 条规则的正则表达式不能为空', { index: i + 1 }));
        return;
      }
      try {
        new RegExp(r.regex);
      } catch (e) {
        showError(
          t('第 {{index}} 条规则的正则表达式无效: {{error}}', {
            index: i + 1,
            error: e.message,
          }),
        );
        return;
      }
      if (!r.body.trim()) {
        showError(t('第 {{index}} 条规则的返回内容不能为空', { index: i + 1 }));
        return;
      }
    }

    const requests = [];
    const newRulesJson = JSON.stringify(rules);

    if (enabled !== snapRef.current.enabled) {
      requests.push(
        API.put('/api/option/', {
          key: 'ua_match_setting.enabled',
          value: String(enabled),
        }),
      );
    }
    if (newRulesJson !== snapRef.current.rulesJson) {
      requests.push(
        API.put('/api/option/', {
          key: 'ua_match_setting.rules',
          value: newRulesJson,
        }),
      );
    }

    if (!requests.length) {
      showWarning(t('你似乎并没有修改什么'));
      return;
    }

    setLoading(true);
    try {
      const results = await Promise.all(requests);
      if (results.some((r) => r === undefined)) {
        showError(t('部分保存失败，请重试'));
        return;
      }
      showSuccess(t('保存成功'));
      snapRef.current = { enabled, rulesJson: newRulesJson };
      props.refresh();
    } catch {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  }

  // ---- 渲染 ----

  return (
    <div>
      <Banner
        type='info'
        description={t(
          '通过正则匹配 User-Agent 请求头，对匹配的请求返回自定义内容。可用于提示使用错误客户端的用户。规则按顺序优先匹配。',
        )}
        style={{ marginBottom: 16 }}
      />

      {/* 开关 */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          marginBottom: 16,
        }}
      >
        <Switch
          checked={enabled}
          onChange={(v) => setEnabled(v)}
          checkedText='ON'
          uncheckedText='OFF'
        />
        <Text>{t('启用 UA 匹配拦截')}</Text>
        <Text type='tertiary' size='small'>
          {t('开启后，匹配到的请求将返回自定义内容')}
        </Text>
      </div>

      {/* 规则列表标题栏 */}
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: 12,
        }}
      >
        <Text strong style={{ fontSize: 15 }}>
          {t('匹配规则列表')}
        </Text>
        <Button
          icon={<IconPlus />}
          size='small'
          onClick={addRule}
          theme='solid'
        >
          {t('添加规则')}
        </Button>
      </div>

      {/* 规则卡片列表 */}
      {rules.length === 0 ? (
        <Empty
          description={t('暂无规则，点击上方按钮添加')}
          style={{ margin: '24px 0' }}
        />
      ) : (
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            gap: 12,
            marginBottom: 16,
          }}
        >
          {rules.map((rule, index) => (
            <Card
              key={index}
              bodyStyle={{ padding: 16 }}
              style={{ border: '1px solid var(--semi-color-border)' }}
            >
              {/* 卡片标题行 */}
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  marginBottom: 12,
                }}
              >
                <Text strong>
                  {t('规则 {{index}}', { index: index + 1 })}
                  {rule.name ? ` — ${rule.name}` : ''}
                </Text>
                <Space size='small'>
                  <Button
                    icon={<IconArrowUp />}
                    size='small'
                    disabled={index === 0}
                    onClick={() => moveRule(index, -1)}
                    theme='borderless'
                  />
                  <Button
                    icon={<IconArrowDown />}
                    size='small'
                    disabled={index === rules.length - 1}
                    onClick={() => moveRule(index, 1)}
                    theme='borderless'
                  />
                  <Popconfirm
                    title={t('确认删除此规则？')}
                    onConfirm={() => removeRule(index)}
                  >
                    <Button
                      icon={<IconDelete />}
                      size='small'
                      type='danger'
                      theme='borderless'
                    />
                  </Popconfirm>
                </Space>
              </div>

              {/* 字段表单 */}
              <Row gutter={[12, 12]}>
                <Col xs={24} sm={8}>
                  <div>
                    <Text type='tertiary' size='small'>
                      {t('规则名称（可选）')}
                    </Text>
                    <Input
                      value={rule.name}
                      onChange={updateRule.bind(null, index, 'name')}
                      placeholder={t('例如：拦截旧版客户端')}
                      style={{ marginTop: 4 }}
                    />
                  </div>
                </Col>
                <Col xs={24} sm={16}>
                  <div>
                    <Text type='tertiary' size='small'>
                      {t('User-Agent 正则表达式')}
                    </Text>
                    <Input
                      value={rule.regex}
                      onChange={(v) => updateRule(index, 'regex', v)}
                      placeholder={t('例如：^Claude/.*$')}
                      style={{ marginTop: 4 }}
                    />
                  </div>
                </Col>
                <Col xs={24} sm={6}>
                  <div>
                    <Text type='tertiary' size='small'>
                      {t('HTTP 状态码')}
                    </Text>
                    <InputNumber
                      value={rule.status_code || 200}
                      onChange={(v) =>
                        updateRule(index, 'status_code', v ?? 200)
                      }
                      min={100}
                      max={599}
                      style={{ width: '100%', marginTop: 4 }}
                    />
                  </div>
                </Col>
                <Col xs={24} sm={18}>
                  <div>
                    <Text type='tertiary' size='small'>
                      {t('返回内容（支持纯文本或 JSON）')}
                    </Text>
                    <Input
                      value={rule.body}
                      onChange={(v) => updateRule(index, 'body', v)}
                      placeholder={t(
                        '例如：{"error": "请升级您的客户端"}',
                      )}
                      style={{ marginTop: 4 }}
                    />
                  </div>
                </Col>
              </Row>
            </Card>
          ))}
        </div>
      )}

      {/* 保存按钮 */}
      <Row>
        <Button
          size='default'
          theme='solid'
          loading={loading}
          onClick={onSubmit}
        >
          {t('保存 UA 匹配设置')}
        </Button>
      </Row>
    </div>
  );
}
