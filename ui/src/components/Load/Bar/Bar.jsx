import React from 'react';
import {Line} from 'react-chartjs-2';
import {Chart} from 'chart.js';
import PropTypes from 'prop-types';
import zoomPlugin from 'chartjs-plugin-zoom';
import 'chartjs-adapter-luxon';

Chart.register(zoomPlugin)

const Bar = ({coin, labelData}) => {
    const data = canvas => {
        const ctx = canvas.getContext('2d');
        const gradient = ctx.createLinearGradient(63, 81, 181, 700);
        gradient.addColorStop(0, '#929dd9');
        gradient.addColorStop(1, '#172b4d');

        console.log(labelData)

        return {
            // labels:labelData,
            datasets: [
                // {
                //     type: "line",
                //     label: coin + "-trades",
                //     data: tradeData,
                //     backgroundColor: gradient,
                //     borderColor: '#3F51B5',
                //     pointRadius: 1,
                //     pointHoverRadius: 2,
                //     pointHoverBorderColor: 'white',
                // },
                {
                    type: "scatter",
                    label: coin + "-history",
                    fill: false,
                    data: labelData,
                    backgroundColor: gradient,
                    borderColor: '#3F51B5',
                    pointRadius: 1,
                    pointHoverRadius: 2,
                    pointHoverBorderColor: 'white',
                }
            ]
        };
    };

    const options = {
        responsive: true,
        scales: {
            x : {
                type: 'time',
                unit: 'day',
                distribution: "linear",
                time: {
                    // Luxon format string
                    tooltipFormat: 'DD T'
                },
            },
        },
        tooltips: {
            titleFontSize: 13,
            bodyFontSize: 13
        },
        plugins: {
            zoom: {
                zoom: {
                    wheel: {
                        enabled: true,
                    },
                    drag: {
                        enabled : true
                    },
                    mode: 'x',
                },
                pan: {
                    enabled: true,
                },
                mode : 'x'
            }
        }
    };

    return (
        <>
            <Line data={data} options={options}/>
        </>
    );
};

Bar.propTypes = {
    labelData: PropTypes.array,
    bmiData: PropTypes.array
};

export default Bar;
