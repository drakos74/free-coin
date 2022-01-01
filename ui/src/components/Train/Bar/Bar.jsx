import React from 'react';
import {Line} from 'react-chartjs-2';
import {Chart} from 'chart.js';
import PropTypes from 'prop-types';
import zoomPlugin from 'chartjs-plugin-zoom';
import 'chartjs-adapter-luxon';

Chart.register(zoomPlugin)

const Bar = ({coin, labelData, tradeData, priceData, ml}) => {
    const data = canvas => {
        const ctx = canvas.getContext('2d');
        const gradient = ctx.createLinearGradient(63, 81, 181, 700);
        gradient.addColorStop(0, '#929dd9');
        gradient.addColorStop(1, '#172b4d');

        // generate the datasets
        let datasets = [{
            type: "line",
            label: coin + "-price",
            fill: false,
            data: priceData,
            backgroundColor: gradient,
            borderColor: '#3F51B5',
            pointRadius: 1,
            pointHoverRadius: 2,
            pointHoverBorderColor: 'white',
        }]

        if (ml) {
            Object.keys(ml).forEach(k => {
                datasets.push({
                    type: "scatter",
                    label: k + "-buy",
                    data: ml[k].buy,
                    backgroundColor: gradient,
                    borderColor: '#3bcb5d',
                    pointRadius: 3,
                    pointHoverRadius: 2,
                    pointHoverBorderColor: 'white',
                })
                datasets.push({
                    type: "scatter",
                    label: k + "-sell",
                    data: ml[k].sell,
                    backgroundColor: gradient,
                    borderColor: '#cb3b3b',
                    pointRadius: 3,
                    pointHoverRadius: 2,
                    pointHoverBorderColor: 'white',
                })
            })
        }

        return {
            datasets :datasets
        };
    };

    const options = {
        responsive: true,
        scales: {
            x: {
                type: 'time',
                distribution: "linear",
                time: {
                    // Luxon format string
                    tooltipFormat: 'MM DD T'
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
                        enabled: true
                    },
                    mode: 'x',
                },
                pan: {
                    enabled: true,
                },
                mode: 'x'
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
