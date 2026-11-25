/** MEPER Studio app tile, sourced from MEPER-Studio-Logo/meper-studio-app.svg. */
import Svg, { Defs, LinearGradient, Stop, Path, Rect, G } from "react-native-svg";

interface SiliecoLogoProps {
  size?: number;
}

export function SiliecoLogo({ size = 48 }: SiliecoLogoProps) {
  return (
    <Svg width={size} height={size} viewBox="0 0 256 256">
      <Defs>
        <LinearGradient id="tile" x1="0%" y1="0%" x2="100%" y2="100%">
          <Stop stopColor="#248DF2" />
          <Stop offset="1" stopColor="#123DAE" />
        </LinearGradient>
      </Defs>
      <Rect width="256" height="256" rx="58.112" fill="url(#tile)" />
      <G transform="translate(35.84 32.256) scale(1.7715199999999998)">
        <Path fill="#FFFFFF" d="M7 86V24L20 17L46 43L34 55L27 48V86Z" />
        <Path fill="#FFFFFF" d="M41 61L83 17L96 24V86H76V49L55 75Z" />
      </G>
    </Svg>
  );
}
