import { USER_NAME } from "../data.js";

function Header({ onMenuClick }) {
  return (
    <header className="header">
      <div className="header-left">
        <button className="burger" onClick={onMenuClick} aria-label="toggle menu">
          <span></span>
          <span></span>
          <span></span>
        </button>
        <h1 className="logo">DeployDock</h1>
      </div>

      <div className="header-right">
        <span>Hello, {USER_NAME}</span>
        <div className="avatar">{USER_NAME[0]}</div>
      </div>
    </header>
  );
}

export default Header;
