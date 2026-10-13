const links = [
  { id: "dashboard", label: "Dashboard" },
  { id: "deployments", label: "Deployments" },
  { id: "budget", label: "Budget" },
];

function Sidebar({ current, onChange }) {
  return (
    <nav className="sidebar">
      {links.map((l) => (
        <a
          key={l.id}
          href={"#" + l.id}
          className={current === l.id ? "active" : ""}
          onClick={(e) => {
            e.preventDefault();
            onChange(l.id);
          }}
        >
          {l.label}
        </a>
      ))}
    </nav>
  );
}

export default Sidebar;
