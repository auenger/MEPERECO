import { dutyAgents, studioWindow } from "@silieco/core/auth/agent-art";

function imageUrl(svg: string) {
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}

const sky = imageUrl(studioWindow);
const art = Object.fromEntries(Object.entries(dutyAgents).map(([name, svg]) => [name, imageUrl(svg)]));

interface SceneProps {
  looking: boolean;
  privateEntry: boolean;
  registering: boolean;
  verifying: boolean;
}

export function DutyAgents({ looking, privateEntry, registering, verifying }: SceneProps) {
  return (
    <div className="duty-scene" aria-hidden="true" data-looking={looking} data-private={privateEntry}>
      <img className="duty-sky" src={sky} alt="" />
      <div className="duty-stage">
        {(["terminal", "courier"] as const).map((agent) => (
          <div className={`duty-actor duty-${agent}`} key={agent}>
            <div className="duty-turn">
              <img className="duty-front" src={art[agent]} alt="" />
              <img className="duty-back" src={art[`${agent}Back`]} alt="" />
            </div>
          </div>
        ))}
        {registering && !verifying && <img className="duty-item duty-badge" src={art.badge} alt="" />}
        {verifying && <img className="duty-item duty-letter" src={art.envelope} alt="" />}
      </div>
    </div>
  );
}
