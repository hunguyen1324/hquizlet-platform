import React from "react";

/** A failed lazy import must leave a recovery action, including after a deployment. */
export class AsyncPageBoundary extends React.Component<React.PropsWithChildren, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  render() {
    if (this.state.failed) return <main className="empty-state" role="alert">
      <h2>Không tải được trang</h2>
      <p>Kiểm tra kết nối rồi tải lại trang để tiếp tục.</p>
      <button type="button" className="primary-button" onClick={() => window.location.reload()}>Tải lại trang</button>
    </main>;
    return this.props.children;
  }
}
