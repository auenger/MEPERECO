import React from 'react';
import {Composition,registerRoot} from 'remotion';
import {CollaborationFilm} from './collaboration-film';
registerRoot(() => <Composition id="MEPERStudioCollaboration" component={CollaborationFilm} durationInFrames={2280} fps={30} width={1920} height={1080}/>);
