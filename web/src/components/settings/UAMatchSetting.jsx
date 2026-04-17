import React, { useEffect, useState } from 'react';
import { Card, Spin } from '@douyinfe/semi-ui';
import SettingsUAMatch from '../../pages/Setting/UAMatch/SettingsUAMatch';
import { API, showError } from '../../helpers';

const UAMatchSetting = () => {
  const [options, setOptions] = useState(null);
  const [loading, setLoading] = useState(false);

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      const map = {};
      data.forEach((item) => {
        map[item.key] = item.value;
      });
      setOptions(map);
    } else {
      showError(message);
    }
  };

  async function onRefresh() {
    try {
      setLoading(true);
      await getOptions();
    } catch (error) {
      showError('刷新失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    onRefresh();
  }, []);

  return (
    <Spin spinning={loading} size='large'>
      <Card style={{ marginTop: '10px' }}>
        {options && <SettingsUAMatch options={options} refresh={onRefresh} />}
      </Card>
    </Spin>
  );
};

export default UAMatchSetting;
