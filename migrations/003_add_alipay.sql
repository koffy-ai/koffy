ALTER TABLE recharge_orders
  MODIFY provider ENUM('wechat', 'alipay') NOT NULL DEFAULT 'wechat';

ALTER TABLE payment_events
  MODIFY provider ENUM('wechat', 'alipay') NOT NULL DEFAULT 'wechat';
