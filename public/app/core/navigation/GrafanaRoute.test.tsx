import React from 'react';
import { render } from '@testing-library/react';
import { GrafanaRoute } from './GrafanaRoute';

describe('GrafanaRoute', () => {
  it('Parses search', () => {
    let capturedProps: any;

    const PageComponent = (props: any) => {
      capturedProps = props;
      return <div />;
    };

    const location = { search: '?query=hello&test=asd' } as any;
    const history = {} as any;
    const match = {} as any;

    render(
      <GrafanaRoute location={location} history={history} match={match} route={{ component: PageComponent } as any} />
    );

    expect(capturedProps.queryParams.query).toBe('hello');
  });
});
